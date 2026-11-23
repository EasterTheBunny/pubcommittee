//go:build !gcp && !local && !emulator && !memory

package main

// A build with no target tag must not produce a binary with no storage. The
// identifier below is deliberately undefined, so compilation fails and the
// message names the fix.
var _ = buildTagRequired_use_one_of_gcp_local_emulator_memory
