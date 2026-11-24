package cloudports_test

import (
	"errors"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/cloudports"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestOpenValidatesWithoutNetwork(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		target cloudports.Target
		env    map[string]string
	}{
		"gcp missing bucket":      {cloudports.GCP, map[string]string{"GCP_PROJECT": "p"}},
		"gcp missing project":     {cloudports.GCP, map[string]string{"GCS_BUCKET": "bucket-1"}},
		"gcp with emulator host":  {cloudports.GCP, map[string]string{"GCP_PROJECT": "p", "GCS_BUCKET": "b-1", "STORAGE_EMULATOR_HOST": "x:1"}},
		"emulator without hosts":  {cloudports.Emulator, map[string]string{}},
		"emulator one host":       {cloudports.Emulator, map[string]string{"STORAGE_EMULATOR_HOST": "x:1"}},
		"unknown target":          {"other", map[string]string{}},
		"gcp bad bucket name":     {cloudports.GCP, map[string]string{"GCP_PROJECT": "p", "GCS_BUCKET": "Bad Name"}},
		"gcp with firestore host": {cloudports.GCP, map[string]string{"GCP_PROJECT": "p", "GCS_BUCKET": "b-1", "FIRESTORE_EMULATOR_HOST": "x:1"}},
	} {
		if _, err := cloudports.Open(t.Context(), c.target, env(c.env)); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
