package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

var errFakeRun = errors.New("fake gcloud failure")

// fakeGcloud records every call and answers the account and token queries.
type fakeGcloud struct {
	account string
	calls   [][]string
	failTok bool
}

func (f *fakeGcloud) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if len(args) > 0 && args[0] == "config" {
		return []byte(f.account + "\n"), nil
	}
	if f.failTok {
		return nil, errFakeRun
	}
	return []byte("tok\n"), nil
}

func TestGcloudTokenLoginType(t *testing.T) {
	t.Parallel()
	const aud = "https://admin.example.invalid"
	tests := []struct {
		name    string
		account string
		want    []string
	}{
		{"user", "a@example.invalid", []string{"gcloud", "auth", "print-identity-token"}},
		{"service account", "d@p.iam.gserviceaccount.com", []string{"gcloud", "auth", "print-identity-token", "--audiences=" + aud}},
		{"no account", "", []string{"gcloud", "auth", "print-identity-token"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeGcloud{account: tc.account}
			tok, err := gcloudToken(context.Background(), f.run, aud)
			if err != nil || tok != "tok" {
				t.Fatalf("got %q, %v", tok, err)
			}
			if got := f.calls[len(f.calls)-1]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("token call = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGcloudTokenFailureNamesEnv(t *testing.T) {
	t.Parallel()
	f := &fakeGcloud{account: "a@example.invalid", failTok: true}
	_, err := gcloudToken(context.Background(), f.run, "https://admin.example.invalid")
	if err == nil || !strings.Contains(err.Error(), envToken) {
		t.Fatalf("err = %v, want mention of %s", err, envToken)
	}
}

func TestPubctlTokenWinsOverGcloud(t *testing.T) {
	t.Parallel()
	env := func(k string) string {
		if k == envToken {
			return "env-token"
		}
		return ""
	}
	c, err := newClient("https://admin.example.invalid", env, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGcloud{account: "a@example.invalid"}
	c.run = f.run
	tok, err := c.token(context.Background())
	if err != nil || tok != "env-token" || len(f.calls) != 0 {
		t.Fatalf("tok=%q err=%v calls=%v", tok, err, f.calls)
	}
}

func TestClientUsesGcloudForRemoteURL(t *testing.T) {
	t.Parallel()
	c, err := newClient("https://admin.example.invalid/", func(string) string { return "" }, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGcloud{account: "d@p.iam.gserviceaccount.com"}
	c.run = f.run
	if _, err := c.token(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"gcloud", "auth", "print-identity-token", "--audiences=https://admin.example.invalid"}
	if got := f.calls[len(f.calls)-1]; !reflect.DeepEqual(got, want) {
		t.Fatalf("token call = %v, want %v", got, want)
	}
}
