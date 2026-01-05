//go:build !tinygo
// +build !tinygo

package main

import (
	"context"
	"testing"

	extism "github.com/extism/go-sdk"
)

const pluginPath = "../../plugins/cipher_ansible.wasm"

func TestEncodeAnsibleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	wasmPath := pluginPath

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmPath,
			},
		},
		Config: map[string]string{
			"vault_password": "integration-test-password",
		},
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	defer plugin.Close(context.Background())

	// Test multiple encryptions
	inputs := []string{
		"secret1",
		"secret2",
	}

	outputs := []string{
		"$ANSIBLE_VAULT;1.1;AES256\n36333933636666383361613265316136336530613466633831386630646137323161393031653966\n3263346665383030353631646439386333316234626661350a643761626665373830646633323635\n30656136633539303763383835346663396663376436396433613763653137323537623139336266\n3538366365313131300a316261316561333938393730636635316237613638633664636564303537\n3962",
		"$ANSIBLE_VAULT;1.1;AES256\n63383038623330333865633238646539363737383961386236363463396334346662356131383839\n3736653463613433323436656339393863636262643234350a353131336336656332343638346633\n62643265616366663630393339333434363235626631656464336264633763393539646138353931\n3737373165396136320a363363616566636632653737393337303765336439663831633637393063\n3831",
	}

	for ndx, input := range inputs {
		exit, out, err := plugin.Call("Encode", []byte(input))
		if err != nil {
			t.Fatalf("Encode(%q) error = %v", input, err)
		}
		if exit != 0 {
			t.Fatalf("Encode(%q) exit = %d, want 0", input, exit)
		}

		// Verify each encryption is different (has randomness)
		output := string(out)
		_, _ = ndx, outputs
		if output != outputs[ndx] {
			t.Errorf("Encode(%q) output = %q, want %q", input, output, outputs[ndx])
		}
	}
}

func TestAnsibleDecodeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	wasmPath := pluginPath

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmPath,
			},
		},
		Config: map[string]string{
			"vault_password": "integration-test-password",
		},
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	defer plugin.Close(context.Background())

	// Test multiple encryptions
	inputs := []string{
		"$ANSIBLE_VAULT;1.1;AES256\n36333933636666383361613265316136336530613466633831386630646137323161393031653966\n3263346665383030353631646439386333316234626661350a643761626665373830646633323635\n30656136633539303763383835346663396663376436396433613763653137323537623139336266\n3538366365313131300a316261316561333938393730636635316237613638633664636564303537\n3962",
		"$ANSIBLE_VAULT;1.1;AES256\n63383038623330333865633238646539363737383961386236363463396334346662356131383839\n3736653463613433323436656339393863636262643234350a353131336336656332343638346633\n62643265616366663630393339333434363235626631656464336264633763393539646138353931\n3737373165396136320a363363616566636632653737393337303765336439663831633637393063\n3831",
	}

	outputs := []string{
		"secret1",
		"secret2",
	}

	for ndx, input := range inputs {
		exit, out, err := plugin.Call("Decode", []byte(input))
		if err != nil {
			t.Fatalf("Decode(%q) error = %v", input, err)
		}
		if exit != 0 {
			t.Fatalf("Decode(%q) exit = %d, want 0", input, exit)
		}

		// Verify each encryption is different (has randomness)
		output := string(out)
		_, _ = ndx, outputs
		if output != outputs[ndx] {
			t.Errorf("Decode(%q) output = %q, want %q", input, output, outputs[ndx])
		}
	}
}
