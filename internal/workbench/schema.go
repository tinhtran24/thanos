package workbench

import _ "embed"

// Schema is the Phase 1 SQLite schema for the local-first workbench state.
//
// It is intentionally driver-agnostic so the desktop backend, web backend, or
// tests can execute it with their selected SQLite driver without hardcoding a
// provider in the domain package.
//
//go:embed schema.sql
var Schema string
