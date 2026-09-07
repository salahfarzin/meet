# Cursor (keyset) pagination for meet listings

## Problem

`meet`'s `GetAll`/`QueryMeets` paginates with `LIMIT ? OFFSET ?`
(`internal/meets/repository.go::queryMeetsPaginated`). `OFFSET` forces MySQL
to scan and discard `offset` rows before returning a page - cost grows
linearly with page depth, expensive on large tables. `uuid` is already v7
(time-ordered) and the table's clustered PK (migration 000008), which makes
it a natural seek key for keyset pagination instead.

## Approach: dual-mode, per caller

Two pagination modes coexist on the same RPC/route, selected by which params
the caller sends:

- **cursor mode** (new): caller sends `cursor`, gets `next_cursor` back.
  Forward-seek only, no arbitrary page jump, no OFFSET.
- **offset mode** (existing, unchanged): caller sends `page`/`page_size`,
  gets numbered pages back exactly as today.

This is deliberate, not a migration shim: ravanhealth's admin list
(`admin/paginate.php`) renders numbered page links, which needs random
access to page N - something keyset pagination cannot do cheaply. That
caller stays on OFFSET permanently. Cursor mode is for the cases that
actually hit "huge data": dashboard's scheduling list and ravanhealth's
full-export while-loop.

## Cursor design

- Cursor = opaque base64 token encoding `{sort_value, uuid}` of the last row
  on the current page. `uuid` is the tiebreaker for rows sharing the same
  `sort_value` (`created_at`/`start_time`/`end_time` are not unique).
- Query shape: `WHERE (sort_col, uuid) > (?, ?) ORDER BY sort_col, uuid
  LIMIT ?` - a composite seek predicate, replacing `LIMIT ? OFFSET ?`.
- `sort_by`/`sort_dir` still select the column, same allow-list
  (`sortableColumns` in repository.go) - cursor mode is not restricted to
  `created_at`.
- `COUNT(*)` on the WHERE clause (minus the seek predicate) is kept and run
  once per request, same as today. This is not a regression: COUNT alone was
  already cheap: the expensive part being removed is the OFFSET row-scan,
  not the count.
- `has_more bool` also returned, so callers can disable Next without
  recomputing from total.
- Malformed/stale cursor (decode failure, or referenced row deleted) ->
  treated as "start from beginning", not an error.

## New index

No composite index today supports the seek. New migration adds, per
sortable column:

```sql
ALTER TABLE meets
  ADD INDEX idx_organizer_created_uuid (organizer_uuid, created_at, uuid),
  ADD INDEX idx_organizer_start_uuid   (organizer_uuid, start_time, uuid),
  ADD INDEX idx_organizer_end_uuid     (organizer_uuid, end_time, uuid);
```

## Proto changes (`proto/meets/meets.proto`)

- Add `string cursor = 14;` to the request (alongside existing `page = 9`,
  `page_size = 10`, `sort_by = 12`, `sort_dir = 13`).
- Add `string next_cursor` and `bool has_more` to the response, alongside
  existing `total`.
- `make generate-proto` after the change.

## Scope (3 repos)

1. **meet**
   - `internal/meets/repository.go`: new `queryMeetsKeyset` path branching
     off `QueryMeets` when `options.Cursor != ""`; cursor encode/decode
     helpers (base64 of `sort_value|uuid`).
   - `internal/meets/service.go`: cursor validation, malformed-cursor
     fallback.
   - New migration for the three composite indexes above.

2. **dashboard**
   - `src/services/appointmentsService.ts::fetchSchedulingList` (currently
     `page`/`pageSize`, appointmentsService.ts:186-217): switch to
     `cursor`/`next_cursor`.
   - `src/components/scheduling/SchedulingView.tsx`: drop
     `totalPages`/page-number state (currently lines 87, 154-161, 219,
     276-299), replace with `next_cursor` + Prev/Next. Prev needs a small
     client-side cursor stack (push visited cursors, pop on Prev) since
     keyset is forward-seek-only. `total` stays for the "N results" label
     (COUNT is still returned).
   - `fetchAppointments` (calendar tab, unpaginated today) - out of scope,
     unaffected.

3. **ravanhealth**
   - `Admin\AppointmentsController` (`indexAppointments` and the two
     participant-scoped lists) - **unchanged**, stays on `page`/`page_size`;
     numbered-page UI (`admin/paginate.php`) requires it.
   - `Admin\AppointmentsController.php:1187` full-export while-loop
     (`page_size=200`, increments `page`) - switches to `cursor`/
     `next_cursor`, loop condition becomes `has_more`. This is the concrete
     "huge data" case the whole change targets.
   - `Index\AppointmentsController`'s two hardcoded `page=1` single-shot
     calls (`fetchAndBucketOpenMeets`, `fetchMyAppointments`) - unaffected
     either way, left as-is.

## Testing

- `internal/meets/repository_test.go`: keyset seek query cases including
  duplicate-`sort_value` tie-breaking across all three sortable columns;
  malformed-cursor fallback.
- `internal/meets/service_test.go`: cursor encode/decode round-trip.
- dashboard: `SchedulingView` test updated for cursor-based Prev/Next state
  instead of page-number math.
- ravanhealth: export while-loop test updated for `has_more`/`cursor` loop
  condition.

## Out of scope

- No change to ravanhealth admin's numbered-page UI or its OFFSET query
  path.
- No back-compat cursor format versioning - single opaque format, callers
  never inspect its contents.
- licensegen service (`~/go/src/github.com/salahfarzin/licensegen`) also
  uses plain Limit/Offset (`repositories/license_repository.go:30-31`) but
  is not touched by this change - no precedent existed there to reuse, and
  porting this pattern to it is a separate effort if ever needed.
