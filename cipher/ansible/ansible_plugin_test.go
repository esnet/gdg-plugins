//go:build !tinygo
// +build !tinygo

package main

import (
	"context"
	"strings"
	"testing"

	extism "github.com/extism/go-sdk"
)

const pluginPath = "../../plugins/cipher_ansible.wasm"

// newTestPlugin builds a fresh extism.Plugin backed by the compiled
// cipher_ansible.wasm, with the given config map applied verbatim (an
// empty/nil map, or one missing "vault_password", exercises the
// no-password error path).
func newTestPlugin(t *testing.T, config map[string]string) *extism.Plugin {
	t.Helper()
	ctx := context.Background()

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: pluginPath},
		},
		Config: config,
	}

	pluginConfig := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, pluginConfig, []extism.HostFunction{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	t.Cleanup(func() { plugin.Close(context.Background()) })
	return plugin
}

// TestAnsibleEncryptDecryptRoundTrip verifies Encode/Decode round-trip
// correctly. It intentionally does NOT assert against a fixed expected
// ciphertext: ansible-vault-go's Encrypt generates a random salt per call
// (see sosedoff/ansible-vault-go's generateRandomBytes), so the exact
// ciphertext is never the same twice — only the round trip is stable.
func TestAnsibleEncryptDecryptRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"vault_password": "integration-test-password"})

	inputs := []string{"secret1", "secret2", "a longer secret value with spaces and punctuation!"}

	for _, input := range inputs {
		exit, ciphertext, err := plugin.Call("Encode", []byte(input))
		if err != nil {
			t.Fatalf("Encode(%q) error = %v", input, err)
		}
		if exit != 0 {
			t.Fatalf("Encode(%q) exit = %d, want 0 (err=%q)", input, exit, plugin.GetError())
		}

		exit, plaintext, err := plugin.Call("Decode", ciphertext)
		if err != nil {
			t.Fatalf("Decode(%q) error = %v", ciphertext, err)
		}
		if exit != 0 {
			t.Fatalf("Decode(%q) exit = %d, want 0 (err=%q)", ciphertext, exit, plugin.GetError())
		}

		if string(plaintext) != input {
			t.Errorf("round trip mismatch: got %q, want %q", plaintext, input)
		}
	}
}

// TestAnsibleEncode_ProducesDifferentCiphertextEachTime verifies the
// randomness claim the previous version of this test only asserted in a
// comment: encoding the same input twice must not produce identical
// ciphertext (a random salt is mixed into every encryption).
func TestAnsibleEncode_ProducesDifferentCiphertextEachTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"vault_password": "integration-test-password"})

	_, first, err := plugin.Call("Encode", []byte("same-input"))
	if err != nil {
		t.Fatalf("first Encode error = %v", err)
	}
	_, second, err := plugin.Call("Encode", []byte("same-input"))
	if err != nil {
		t.Fatalf("second Encode error = %v", err)
	}

	if string(first) == string(second) {
		t.Fatalf("expected two encryptions of the same input to differ (random salt), got identical ciphertext")
	}
}

// TestAnsibleEncode_MissingPasswordReturnsError verifies a clean, non-panic
// error when no vault_password is configured.
func TestAnsibleEncode_MissingPasswordReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{})

	exit, _, err := plugin.Call("Encode", []byte("secret"))
	if exit == 0 {
		t.Fatalf("Call(Encode) exit = 0, want non-zero when no vault_password is configured")
	}
	if err == nil {
		t.Fatalf("Call(Encode) expected an error when no vault_password is configured, got nil")
	}
	if !strings.Contains(err.Error(), "vault password") {
		t.Fatalf("expected error mentioning a missing vault password, got %q", err)
	}
}

// TestAnsibleDecode_MissingPasswordReturnsError mirrors the Encode case
// for Decode.
func TestAnsibleDecode_MissingPasswordReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{})

	exit, _, err := plugin.Call("Decode", []byte("$ANSIBLE_VAULT;1.1;AES256\nirrelevant"))
	if exit == 0 {
		t.Fatalf("Call(Decode) exit = 0, want non-zero when no vault_password is configured")
	}
	if err == nil {
		t.Fatalf("Call(Decode) expected an error when no vault_password is configured, got nil")
	}
	if !strings.Contains(err.Error(), "vault password") {
		t.Fatalf("expected error mentioning a missing vault password, got %q", err)
	}
}

// TestAnsibleDecode_InvalidFormatReturnsError verifies that malformed vault
// content (missing the "$ANSIBLE_VAULT;..." header) is rejected cleanly
// with a correctly configured password, rather than panicking.
func TestAnsibleDecode_InvalidFormatReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"vault_password": "integration-test-password"})

	exit, _, err := plugin.Call("Decode", []byte("this is not a valid ansible vault payload"))
	if exit == 0 {
		t.Fatalf("Call(Decode) exit = 0, want non-zero for malformed vault content")
	}
	if err == nil {
		t.Fatalf("expected a non-empty error for malformed vault content")
	}
}
