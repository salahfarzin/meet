# Meet domain

The proto (`proto/meets/meets.proto`) is the source of truth for the API contract. Regenerate stubs with `make generate-proto` after any change.

## Entity

`Meet` (proto) ↔ `meets.Meet` (Go struct, `internal/meets/repository.go`) ↔ `meets` table (`migrations/000001_*`):

| field | notes |
|-------|-------|
| `uuid` | public id (google/uuid); table also has internal autoinc `id` |
| `organizer_uuid` | owner; derived from auth context, not trusted from client |
| `price_uuid` | nullable |
| `participant_uuids` | JSON column; repeated string in proto |
| `type` | `MeetType` enum: UNSPECIFIED, IMMEDIATE_PHONE_CALL, CHAT, PHONE_CALL, VIDEO_CALL |
| `start` / `end` | `start_time`/`end_time` DATETIME; proto exposes RFC3339 strings |
| `title`, `description`, `color` | |
| `booked_at` | nullable timestamp |

## RPCs / REST routes (`MeetService`)

| RPC | REST | purpose |
|-----|------|---------|
| `GetAll` | `GET /meets` | list, optional `organizer_id` + `from`/`to` (RFC3339) range filter |
| `GetOne` | `GET /meets/{uuid}` | single |
| `Create` | `POST /meets` (body=meet) | create; conflict-checked |
| `Update` | `PUT /meets/{uuid}` (body=meet) | update; conflict-checked |
| `Delete` | `DELETE /meets/{uuid}` | delete |
| `GetAvailability` | `GET /meets/{uuid}/availability` | bookable slots, defaults to next 7 days; `from`/`to` are `YYYY-MM-DD` |
| `GetMeetTypes` | `GET /meets/{uuid}/types` | enumerate meet types |

## Business rules (service.go)

- **Conflict check** on Create/Update: `repo.HasConflict(organizer, start, end)` rejects overlapping meets for the same organizer → returns `"appointment conflict for this organizer and period"`, surfaced as gRPC `InvalidArgument`.
- Time parsing centralized in `service.ParseStartAndEndTimes`.
- Availability generation lives in `repository.GenerateAvailableSlots` + `service.GetAvailability` (builds per-day `DateSlot`/`TimeSlot`).

## JSON serialization quirk

The gateway marshals with `UseProtoNames: true` → responses are **snake_case** (`organizer_uuid`, `participant_uuids`). Unmarshal uses `DiscardUnknown: true`, so unknown request fields are silently dropped — watch for client/server field-name drift.
