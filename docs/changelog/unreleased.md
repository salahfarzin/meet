# Unreleased

Changes merged into `dev` after [v1.4.0](v1.4.0.md). When tagging, rename this file to the
new version, set the heading and date, and start a fresh `unreleased.md`.

## Fixes

- Super admins and admins got no elevated access. Roles arrive from auth's `/me` as slugs
  (`super-admin`, `admin`), but meet compared them with psychometrist's old stored names
  (`Programmer`, `Admin`), so they never matched: a super admin couldn't set another
  organizer, see every organizer's meets, or manage specialties. `RoleSuperAdmin` and
  `RoleAdmin` are now the slugs.
