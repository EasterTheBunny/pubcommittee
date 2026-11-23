//go:build gcp || emulator

package main

import "errors"

var errNoCloud = errors.New("cloud adapters arrive with M1-03b")
