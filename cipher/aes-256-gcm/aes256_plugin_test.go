//go:build !tinygo
// +build !tinygo

package main

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	extism "github.com/extism/go-sdk"
)

const pluginPath = "../../plugins/cipher_aes256_gcm.wasm"

// newTestPlugin builds a fresh extism.Plugin backed by the compiled
// cipher_aes256_gcm.wasm, with the given config map applied verbatim (an
// empty/nil map, or one missing "passphrase", exercises the
// no-passphrase error path).
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

// TestAES256EncryptDecryptRoundTrip verifies Encode/Decode round-trip
// correctly. It intentionally does NOT assert against a fixed expected
// ciphertext: Encode generates a random salt and nonce per call (see
// plugin.go's use of crypto/rand), so the exact ciphertext is never the
// same twice — only the round trip is stable.
func TestAES256EncryptDecryptRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"passphrase": "integration-test-password"})

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

// TestAES256Encode_ProducesDifferentCiphertextEachTime verifies the
// randomness claim the previous version of this test only asserted in a
// comment: encoding the same input twice must not produce identical
// ciphertext (a random salt and nonce are mixed into every encryption).
func TestAES256Encode_ProducesDifferentCiphertextEachTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"passphrase": "integration-test-password"})

	_, first, err := plugin.Call("Encode", []byte("same-input"))
	if err != nil {
		t.Fatalf("first Encode error = %v", err)
	}
	_, second, err := plugin.Call("Encode", []byte("same-input"))
	if err != nil {
		t.Fatalf("second Encode error = %v", err)
	}

	if string(first) == string(second) {
		t.Fatalf("expected two encryptions of the same input to differ (random salt/nonce), got identical ciphertext")
	}
}

// TestAES256Encode_MissingPassphraseReturnsError verifies a clean,
// non-panic error when no passphrase is configured.
func TestAES256Encode_MissingPassphraseReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{})

	exit, _, err := plugin.Call("Encode", []byte("secret"))
	if exit == 0 {
		t.Fatalf("Call(Encode) exit = 0, want non-zero when no passphrase is configured")
	}
	if err == nil {
		t.Fatalf("Call(Encode) expected an error when no passphrase is configured, got nil")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected error mentioning a missing passphrase, got %q", err)
	}
}

// TestAES256Decode_MissingPassphraseReturnsError mirrors the Encode case
// for Decode.
func TestAES256Decode_MissingPassphraseReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{})

	exit, _, err := plugin.Call("Decode", []byte(base64.StdEncoding.EncodeToString([]byte("irrelevant-because-passphrase-missing"))))
	if exit == 0 {
		t.Fatalf("Call(Decode) exit = 0, want non-zero when no passphrase is configured")
	}
	if err == nil {
		t.Fatalf("Call(Decode) expected an error when no passphrase is configured, got nil")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("expected error mentioning a missing passphrase, got %q", err)
	}
}

// TestAES256Decode_InvalidBase64ReturnsError verifies that input which
// isn't valid base64 at all is rejected cleanly by the base64-decode step,
// before any cryptography is attempted.
func TestAES256Decode_InvalidBase64ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"passphrase": "integration-test-password"})

	exit, _, err := plugin.Call("Decode", []byte("not-valid-base64!!!"))
	if exit == 0 {
		t.Fatalf("Call(Decode) exit = 0, want non-zero for invalid base64 input")
	}
	if err == nil {
		t.Fatalf("expected a non-empty error for invalid base64 input")
	}
}

// TestAES256Decode_TooShortDataReturnsError verifies that base64-valid
// input which decodes to fewer bytes than saltSize+nonceSize is rejected
// as "invalid encrypted data" rather than panicking on a short slice.
func TestAES256Decode_TooShortDataReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	plugin := newTestPlugin(t, map[string]string{"passphrase": "integration-test-password"})

	tooShort := base64.StdEncoding.EncodeToString([]byte("short"))
	exit, _, err := plugin.Call("Decode", []byte(tooShort))
	if exit == 0 {
		t.Fatalf("Call(Decode) exit = 0, want non-zero for too-short encrypted data")
	}
	if err == nil {
		t.Fatalf("Call(Decode) expected an error for too-short encrypted data, got nil")
	}
	if !strings.Contains(err.Error(), "invalid encrypted data") {
		t.Fatalf("expected error mentioning invalid encrypted data, got %q", err)
	}
}
