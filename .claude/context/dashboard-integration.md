# Dashboard integration

This service is the backend for the **events / appointments** section of the `dashboard-v1` React app (`~/projects/dashboard-v1`).

- Dashboard route `events` renders `pages/Appointment.tsx`.
- Dashboard service `src/services/appointmentsService.ts` calls this service's REST routes directly via its `httpClient`:
  - `GET /meets` (with `from`/`to` query filters) → list
  - `GET /meets/{uuid}` → single
  - `POST /meets` → create
  - `PUT /meets/{uuid}` → update
  - `DELETE /meets/{uuid}` → delete
- Dashboard's `Appointment` type maps 1:1 to the proto `Meet`. `CreateResponse` mirrors `{ status: {code, message}, meet }`.

## Cross-repo gotchas

- **Field naming:** dashboard sends camelCase (`participantUuids`, `priceUuid`); this gateway emits snake_case and discards unknown fields on input. protojson accepts both lowerCamel and snake on input, but verify any new field round-trips both ways.
- **Auth:** the dashboard's JWT (issued by `AUTH_SERVICE`) is forwarded as `Bearer`; this service validates it via `AUTH_SERVICE/me`. Both must point at the same auth service.
- **CORS:** dashboard dev origin (`http://localhost:5173`) must be in `CORS_ALLOWED_ORIGINS`.
- The dashboard reads list responses resiliently (`response.meets || response.items || response.return.items`) — the canonical shape from this service is `{ "meets": [...] }`.
