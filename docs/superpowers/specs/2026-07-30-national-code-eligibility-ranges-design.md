# National code eligibility: multi-row ranges

## Problem

Eligibility rules (dashboard Settings > Booking, and per-appointment form)
restrict who can book by national code prefix/suffix, currently a single
`nationalCodeStartsWith` / `nationalCodeEndsWith` string pair. Needs to
become a multi-row list, each row: Title, Start From, Start To, End From,
End To, with add/remove controls.

## Wire contract change

Replace (breaking, no migration — low usage, old fields simply dropped):

```jsonc
"eligibility": {
  "national_code_starts_with": "001",
  "national_code_ends_with": "5"
}
```

with:

```jsonc
"eligibility": {
  "national_code_rules": [
    { "title": "Region A", "start_from": "001", "start_to": "005", "end_from": "", "end_to": "" }
  ]
}
```

Dashboard camelCases at the transport boundary: `nationalCodeRules[].{title,
startFrom, startTo, endFrom, endTo}`.

Rules are OR'd across rows (any row match -> eligible). Within a row, all
non-empty bounds are AND'd.

## Match semantics (ravanhealth)

For a row, take the national code's leading substring of length
`max(len(startFrom), len(startTo))`, zero-pad both bounds to that length,
compare numerically: code's leading substring must be within
`[startFrom, startTo]`. Same idea using the trailing substring for
`endFrom`/`endTo`. A bound pair where both sides are empty is skipped
(no constraint from that half of the row). This generalizes the old
`str_starts_with`/`str_ends_with` exactly when `startFrom == startTo`.

## Scope (3 repos)

1. **dashboard**
   - `src/types/Eligibility.ts`: `NationalCodeRule` type, `nationalCodeRules`
     replaces `nationalCodeStartsWith`/`nationalCodeEndsWith` in
     `EligibilityRules`, update `mapEligibility`/`serializeEligibility`.
   - `src/components/shared/EligibilityRulesAccordion.tsx`: replace the two
     FormFields with a `useFieldArray`-driven row list (reuses `AddRowButton`,
     `FormField` — same pattern as `PriceList.tsx`/
     `RecurringAvailabilityAccordion.tsx`; no new generic component needed).

2. **ravanhealth**
   - `src/Controllers/Admin/Traits/EligibilityRulesTrait.php`: replace the
     two starts/ends-with constants + `getEligibilityValue` calls with array
     parsing for `national_code_rules` (validate each row's 5 sub-fields,
     drop the field-level permission gate to a whole-array gate since it's
     no longer flat fields).
   - `src/Controllers/Index/AppointmentsController.php::meetsEligibilityRules()`:
     replace the starts/ends-with block (lines ~550-557) with the range-match
     loop over `national_code_rules`.

3. **psychometrist**
   - `app/Http/Requests/Api/V1/Concerns/HasUserSettingsRules.php`: replace
     the two `settings.appointments.eligibility.national_code_*` rules with
     `settings.appointments.eligibility.national_code_rules` (array) +
     `.*.title`/`.*.start_from`/`.*.start_to`/`.*.end_from`/`.*.end_to`
     (nullable strings).

## Testing

- dashboard: existing component/type tests updated for new shape (no new
  test infra).
- ravanhealth: `EligibilityRulesTrait`/`AppointmentsController` tests updated
  for range matching (exact-prefix-as-range case + true range case).
- psychometrist: validation test for `national_code_rules` shape (skip if
  local DB blocks running, per existing known-issue precedent in
  `eligibility.md`).

## Out of scope

- No back-compat/migration for existing stored `national_code_starts_with`/
  `ends_with` values.
- No change to `health_card_clinic_id` eligibility (unrelated field, left
  as-is).
