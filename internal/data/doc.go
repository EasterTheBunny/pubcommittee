// Package data holds the domain types, typed errors, and port interfaces
// every PubCommittee service depends on. Nothing here imports a storage SDK.
//
// # Adding a port
//
//  1. Declare the interface in a file named for its group, in domain terms:
//     domain types only, context.Context first, opaque cursors, typed errors.
//  2. Add it to the group struct in groups.go that should hold it.
//  3. Add an exported suite to internal/data/conformance.
//  4. Implement it in internal/adapters/memory and call the suite there.
//  5. Add it to internal/telemetry's port wrapper so operations are timed.
//
// # Adding an adapter
//
// Create internal/adapters/<name>, give its constructor a typed config that
// validates without network calls, implement ports, wrap failures in the
// typed errors of errors.go, and call the conformance suites in its tests.
// Local-only adapters are never imported from a gcp wiring file.
//
// # Adding a service
//
// Create cmd/<service>/ with main.go and one wiring file per build tag
// (gcp, local, emulator, memory) plus wiring_none.go. See cmd/AGENTS.md.
package data
