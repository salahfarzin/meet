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

## Specialties catalogue (`SpecialtyService`, `internal/specialties`)

Admin-managed list trappists pick from for their public profile (dashboard Settings → Appointments → Public Profile; managed in dashboard Events → Specialties). Table `specialties` (`migrations/000003_*`): `uuid` (v7 PK), `kind` (`specialty` | `approach`), `slug` (unique per kind), `names` JSON (`{"en": "...", "fa": "..."}`), `sort_order`, `active`.

| RPC | REST | who |
|-----|------|-----|
| `GetAll` | `GET /meets/specialties?kind=&include_inactive=` | any authenticated caller; `include_inactive` honored for admins only |
| `Create` | `POST /meets/specialties` (body=specialty) | admin / super-admin; always created active |
| `Update` | `PUT /meets/specialties/{uuid}` | admin / super-admin |
| `Delete` | `DELETE /meets/specialties/{uuid}` | admin / super-admin; hard delete |

Trappists store only the uuids, in psychometrist `users.center_details.appointments.profile.{specialties,approaches}`; rawej resolves them against `GetAll` (active only), so hidden or deleted entries drop off patient-facing profiles. Registered after `MeetService` so the literal route beats `/meets/{uuid}`.

## Gooyai integration (`GooyaiService`, `internal/gooyai` + `pkg/gooyai`)

[Gooyai](https://docs.gooyai.ir/docs/intro) is an AI phone receptionist with its own providers, slots and bookings. `pkg/gooyai` is a plain Partner API client (imports nothing from meet, reusable by other services); `internal/gooyai` picks the clinic's key and maps results to our proto. Each clinic connects itself from the dashboard (Settings → Phone assistant): its key is checked against Gooyai, sealed with AES-256-GCM under `GOOYAI_ENCRYPTION_KEY` (base64 of 32 bytes; unset = integration disabled, malformed = refuse to start), and stored **in the clinic's user settings**, not in meet's database: `settings.phone_assistant = {key_ciphertext, key_last4, environment, updated_at}` on the identity (auth) service, which owns users (`USERS_SERVICE`, its base URL — not `AUTH_SERVICE`, which ends in `/auth`). `usersRepository` reads and writes it through `GET`/`PUT /users/{uuid}` **with the caller's own bearer token**, so the users service's rules decide who may touch whose settings; under `AUTH_DISABLED` the placeholder tenant is resolved through `GET /me`. Those settings reach the browser, which is why only ciphertext goes there — the plaintext key exists only inside meet. The identity service must accept `phone_assistant` as a settings key, or it silently drops the write (`Connect` reads it back and fails instead). Sync state stays in `meets.settings` and in Gooyai.

Consequence for Phase 2: a Gooyai webhook carries no user token, so reading the key there needs a service credential on the users service.

| RPC | REST | notes |
|-----|------|-------|
| `ListCalls` | `GET /meets/calls?direction&page&page_size&from&to&phone&should_recall` | `{calls,total,page,page_size,total_pages}`; `page_size` ≤ 100; `from`/`to` Gregorian `YYYY-MM-DD` |
| `GetCall` | `GET /meets/calls/{id}?direction` | returns the call **bare**, not wrapped |

`direction` is `incoming` (the default when empty) or `outgoing`; each call carries it. Gooyai's Partner API only has incoming calls, so `outgoing` is accepted as a known value but returns `UNIMPLEMENTED` (HTTP 501) until it does; anything else is `INVALID_ARGUMENT`. Call ids are only unique within a direction. `pkg/gooyai` keeps Gooyai's own `incoming-calls` naming.
| `ListProviders` | `GET /meets/phone-assistant/providers` | all pages walked server-side |
| `GetConnection` | `GET /meets/phone-assistant/connection` | `{connected, environment, key_last4, updated_at}` — **never the key** |
| `SaveConnection` | `PUT /meets/phone-assistant/connection` (body `{api_key, environment}`) | probes Gooyai first: a rejected key is `INVALID_ARGUMENT` and nothing is stored; replaces any existing key |
| `DeleteConnection` | `DELETE /meets/phone-assistant/connection` | |

The HTTP routes say `phone-assistant`, not the vendor, because the dashboard is white-label: no vendor name appears in its UI, code or the paths it calls. The vendor name stays inside meet (`pkg/gooyai`, `internal/gooyai`, the proto package).

- The tenant is the caller's own uuid; only `super-admin` may pass `organizer_id` (same rule as `retrieveOrganizerUuid`). Never take a tenant or key from the request otherwise.
- Clinic not connected → `FAILED_PRECONDITION` (HTTP 400, `code: 9`). Gooyai rejecting the key is also `FAILED_PRECONDITION`, **never** `UNAUTHENTICATED`: the dashboard treats a 401 as its own session expiring and logs the user out.
- Gooyai's error messages are Persian and never forwarded; map by status (`errors.Is(err, gooyai.ErrNotFound)` etc.).
- `pkg/gooyai` retries GETs on 5xx/transport errors only — never writes, since Gooyai has no idempotency keys and a retried POST could double-book.
- Registered after `MeetService` so the literal routes beat `/meets/{uuid}`.
