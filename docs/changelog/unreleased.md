# Unreleased

Changes merged into `dev` after [v1.4.1](v1.4.1.md). When tagging, rename this file to the
new version, set the heading and date, and start a fresh `unreleased.md`.

## Features

- `GET /release-notes`: this service's release notes (`docs/changelog`, embedded in the binary)
  as JSON, from `utils/releasenotes`: what is unreleased and the latest release. Outside
  `/api/v1`, so nginx doesn't route it: content-manager reads it on the internal network for
  the dashboard's What's new editor. `.dockerignore` keeps `docs/changelog` in the image for it.
