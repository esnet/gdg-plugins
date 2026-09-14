//go:build !tinygo
// +build !tinygo

package main

import (
	"context"
	"strings"
	"testing"

	extism "github.com/extism/go-sdk"
)

const pluginPath = "../../plugins/lookup_gsm.wasm"

// newTestHostFunction builds the get_gcp_access_token host function using
// the exact name/namespace/signature GDG registers in production (see
// internal/adapter/plugins/lookup/gsm/host_functions.go), but backed by a
// fixed test token (or a forced failure) instead of a real oauth2.TokenSource.
// The compiled guest links against this function at instantiation time
// regardless of whether a given test path ends up calling it, so every test
// below must supply one.
func newTestHostFunction(token string, fail bool) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"get_gcp_access_token",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if fail {
				stack[0] = 0
				return
			}
			offset, err := p.WriteString(token)
			if err != nil {
				stack[0] = 0
				return
			}
			stack[0] = offset
		},
		[]extism.ValueType{},
		[]extism.ValueType{extism.ValueTypePTR},
	)
}

func newTestPlugin(t *testing.T, hostFn extism.HostFunction) *extism.Plugin {
	t.Helper()
	ctx := context.Background()

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: pluginPath},
		},
		AllowedHosts: []string{"secretmanager.googleapis.com"},
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{hostFn})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	t.Cleanup(func() { plugin.Close(context.Background()) })
	return plugin
}

// TestLookup_EmptyResourceNameReturnsError verifies the guest rejects an
// empty input before ever calling the host for a token — no network
// involved, so this runs even under `go test -short`.
func TestLookup_EmptyResourceNameReturnsError(t *testing.T) {
	plugin := newTestPlugin(t, newTestHostFunction("unused", false))

	exit, _, err := plugin.Call("Lookup", []byte(""))
	if exit == 0 {
		t.Fatalf("Call(Lookup, \"\") exit = 0, want non-zero")
	}
	if err == nil {
		t.Fatalf("Call(Lookup, \"\") expected an error for an empty resource name, got nil")
	}
	if !strings.Contains(err.Error(), "no secret resource name provided") {
		t.Fatalf("expected error mentioning a missing resource name, got %q", err)
	}
}

// TestLookup_HostTokenFailureReturnsError verifies that when the host's
// get_gcp_access_token function reports failure (offset 0 — see
// handleGetAccessToken on the host side), the guest fails cleanly without
// ever attempting an HTTP request. No network involved.
func TestLookup_HostTokenFailureReturnsError(t *testing.T) {
	plugin := newTestPlugin(t, newTestHostFunction("", true))

	exit, _, err := plugin.Call("Lookup", []byte("projects/example-project/secrets/example-secret/versions/1"))
	if exit == 0 {
		t.Fatalf("Call(Lookup, ...) exit = 0, want non-zero")
	}
	if err == nil {
		t.Fatalf("Call(Lookup, ...) expected an error when the host fails to provide a token, got nil")
	}
	if !strings.Contains(err.Error(), "did not return a GCP access token") {
		t.Fatalf("expected error mentioning a missing access token, got %q", err)
	}
}

// TestLookupIntegration_RealAPICallWithFakeTokenIsRejected exercises the
// full HTTP round trip against the real Secret Manager API using an
// intentionally-invalid bearer token. It requires network access to
// secretmanager.googleapis.com but no real GCP project or credentials —
// Google is expected to reject the fake token, which is exactly the path
// this test verifies: a non-2xx response is surfaced as a clean error
// rather than a panic or a malformed lookup result.
//
// This is the only test in this file that reaches the real network, so it
// is skipped under `go test -short`.
func TestLookupIntegration_RealAPICallWithFakeTokenIsRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test that requires network access")
	}

	plugin := newTestPlugin(t, newTestHostFunction("not-a-real-access-token", false))

	exit, _, err := plugin.Call("Lookup", []byte("projects/example-project/secrets/example-secret/versions/1"))
	if exit == 0 {
		t.Fatalf("Call(Lookup, ...) exit = 0, want non-zero (a fake token must be rejected by the real API)")
	}
	if err == nil {
		t.Fatalf("expected a non-empty error from the rejected request, got nil")
	}
	t.Logf("received expected rejection from Secret Manager: %s", err)
}
