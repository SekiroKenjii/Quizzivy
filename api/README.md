# API contract

`openapi.yaml` lands here in **T-0.7** and is the single source of truth for the
API. Spec §15 documents it; this file is what both sides are generated from.

Nothing here is hand-written twice: `oapi-codegen` produces the Go server
interfaces, `openapi-typescript` produces the TS types, and MSW fixtures are
validated against this schema with `ajv`. CI regenerates and fails on drift.

Change the contract here first, then `make gen`.
