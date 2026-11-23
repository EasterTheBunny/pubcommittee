//go:build gcp || emulator

package main

import "errors"

// errNoCloud is returned until M1-03b adds the cloud adapters and the Google
// ID token verifier.
var errNoCloud = errors.New("cloud adapters arrive with M1-03b")
