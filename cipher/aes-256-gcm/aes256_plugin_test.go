//go:build !tinygo
// +build !tinygo

package main

import (
	"context"
	"testing"

	extism "github.com/extism/go-sdk"
	"github.com/matryer/is"
)

const pluginPath = "../../plugins/cipher_aes256_gcm.wasm"

func TestEncodeAgeIntegration(t *testing.T) {
	assert := is.New(t)
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
			"passphrase": "integration-test-password",
		},
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	assert.True(err == nil)
	defer plugin.Close(ctx)

	// Test multiple encryptions
	inputs := []string{
		"secret1",
		"secret2",
	}

	outputs := []string{
		"Y5PP+Dqi4aY+Ck/IGPDachqQHp8sT+gAVh3ZjLePo4gWJzGkfWkfB6Ch22GgoDb9xhON",
		"MbS/pcgIswOOwo3pZ3iai2ZMnET7WhiJduTKQxV1dyXcemfYD0MXLMSAEvy7WsGc9s8h",
	}

	for ndx, input := range inputs {
		exit, out, plugErr := plugin.Call("Encode", []byte(input))
		assert.True(plugErr == nil)
		assert.True(exit == 0)
		// Verify each encryption is different (has randomness)
		output := string(out)
		_, _ = ndx, outputs
		if output != outputs[ndx] {
			t.Errorf("Encode(%q) output = %q, want %q", input, output, outputs[ndx])
		}

	}
}

func TestDecodeAgeIntegration(t *testing.T) {
	assert := is.New(t)
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
			"passphrase": "integration-test-password",
		},
	}

	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	assert.True(err == nil)
	defer plugin.Close(context.Background())

	// Test multiple encryptions
	inputs := []string{
		"Y5PP+Dqi4aY+Ck/IGPDachqQHp8sT+gAVh3ZjLePo4gWJzGkfWkfB6Ch22GgoDb9xhON",
		"MbS/pcgIswOOwo3pZ3iai2ZMnET7WhiJduTKQxV1dyXcemfYD0MXLMSAEvy7WsGc9s8h",
	}

	outputs := []string{
		"secret1",
		"secret2",
	}

	for ndx, input := range inputs {
		exit, out, plugErr := plugin.Call("Decode", []byte(input))
		if plugErr != nil {
			t.Fatalf("Decode(%q) error = %v", input, plugErr)
		}
		assert.True(exit == 0)

		// Verify each encryption is different (has randomness)
		output := string(out)
		_, _ = ndx, outputs
		if output != outputs[ndx] {
			t.Errorf("Decode(%q) output = %q, want %q", input, output, outputs[ndx])
		}
	}
}
