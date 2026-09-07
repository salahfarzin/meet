# Booking eligibility rules (cross-repo)

Clinic-level and slot-level **eligibility rules** (min age, max age, gender,
national code starts-with/ends-with) restrict who can book an appointment.
Settings reorganized into single `settings.appointments` namespace across
dashboard and psychometrist.

## Wire contract

```jsonc
// User (psychometrist center_details, clinic-wide defaults)
"settings": {
  "appointments": {
    "first_availability": 5,
    "auto_approve_meetings": true,
    "min_hours_between_bookings": 2,
    "prevent_multiple_upcoming_bookings": true,
    "eligibility": {
      "min_age": 18,
      "max_age": 65,
      "gender": "female",
      "national_code_starts_with": "001",
      "national_code_ends_with": "5"
    },
    "notifications": {
      "reminder_hours": [24, 2],
      "notify_on_book": true,
      "notify_on_edit": true,
      "notify_on_cancel": false
    }
  }
}

// Appointment (meet service, Meet.Settings, per-slot override)
"settings": {
  "eligibility": { "min_age": 18, "gender": "female" }
}
```

Dashboard's `httpClient` auto-camelizes responses / auto-snakizes request
bodies at the transport boundary — app code (`types/Eligibility.ts`,
`types/User.ts`) works entirely in camelCase.

## IMPORTANT: meet does NOT enforce eligibility

`internal/meets/settings.go`'s `MeetSettings` deliberately does **not** model
eligibility (wrong-type risk) — comment explains it's caller-enforced.
`GetAvailability`/`GetAll` return slots **unfiltered** by age/gender/national-code.

Only ravanhealth's `Admin\AppointmentsController::filterOpenEligibleSlots()`
applies the rule (clinic rules from `organizer` details AND slot rules from
`meet.settings.eligibility`, both must pass), and only for its own booking
flow. Any other caller hitting meet's REST API directly (dashboard's own
`GET /meets`, or a future client) gets unfiltered results.

This was a deliberate decision, confirmed with the user: asked whether to
move enforcement into meet's Go service (mirroring the `booking_rules.go`
pattern used for `min_hours_between_bookings`/`prevent_multiple_upcoming_bookings`)
— answer was **no, leave as-is**. Revisit only if a non-ravanhealth caller
ever needs eligibility enforced server-side.

## Where things live (other repos)

| Repo | File | Role |
|---|---|---|
| dashboard | `src/types/Eligibility.ts` | `EligibilityRules` type + `mapEligibility`/`serializeEligibility` |
| dashboard | `src/components/shared/EligibilityRulesAccordion.tsx` | reusable form section, reads `namePrefix` via `useFormContext()` |
| dashboard | `src/forms/AppointmentForm.tsx` | slot-level, `namePrefix="settings.eligibility"` |
| dashboard | `src/components/settings/BookingSettings.tsx` | clinic-level, `namePrefix="eligibility"` → `settings.appointments.eligibility` |
| dashboard | `src/types/User.ts`, `src/services/authService.ts` | `settings.appointments.{...}` typed shape |
| psychometrist | `app/Http/Requests/Api/V1/Concerns/HasUserSettingsRules.php` | `allowedSettingsKeys()` (top-level key just `appointments` now) + `settingsRules()` for full nested shape |
| psychometrist | `app/Http/Controllers/Api/V1/Controllers/UserController.php` | `loadTrappists()` reads `settings.appointments.first_availability` |
| ravanhealth | `src/Controllers/Admin/Traits/EligibilityRulesTrait.php` | shared trait (min/max age clamp, field parsing), used by `Admin\AppointmentsController` and `Admin\UsersController` |
| ravanhealth | `src/Controllers/Index/AppointmentsController.php::filterOpenEligibleSlots()` | actual enforcement at booking time |
| ravanhealth | `src/Repositories/UserRepository.php::findDetailsByUuids()` | bulk-fetch clinic `details` blob for eligibility check |

## Known unresolved (as of last handoff)

- psychometrist `UserControllerTest.php` new/updated cases only `php -l`
  syntax-checked, not run (local Docker mysql `DB_PASSWORD` mismatch vs
  `.env.testing`/`phpunit.xml`, pre-existing infra issue).
- ravanhealth `Admin\AppointmentsControllerTest.php` has 10 pre-existing
  failures unrelated to this work, flagged twice, not fixed.
- OpenAPI/Swagger docstrings in psychometrist `UserController.php`
  (`OA\Property` blocks ~line 400/601) still list old flat
  `first_availability` — cosmetic only, not updated.
