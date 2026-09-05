//go:build tinygo
// +build tinygo

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/extism/go-pdk"
)

// get_gcp_access_token is a host function provided by GDG (see
// internal/adapter/plugins/lookup/gsm/host_functions.go on the host side).
// It returns the offset of a freshly minted GCP OAuth2 access token written
// into this plugin's own memory, or 0 if the host failed to obtain one (the
// underlying cause is logged host-side, not passed back over this simple
// stack-based ABI). Declared with the classic TinyGo import pragmas (rather
// than //go:wasmimport) to match extism's own reference guest examples for
// the "-target wasi" TinyGo build this repo uses.
//
//go:wasm-module extism:host/user
//export get_gcp_access_token
func get_gcp_access_token() uint64

// secretManagerBaseURL is the Google Secret Manager REST API endpoint this
// plugin talks to. The host-side manifest's AllowedHosts (set in
// gsm_lookup.go on the GDG side) restricts this plugin's outbound HTTP
// access to exactly this host — any other destination is rejected by the
// extism runtime before the request ever leaves the process.
const secretManagerBaseURL = "https://secretmanager.googleapis.com/v1/"

// accessSecretVersionResponse mirrors the fields of Secret Manager's
// AccessSecretVersionResponse that this plugin actually needs. The real
// response also includes "name" and "payload.dataCrc32c", both ignored
// here.
type accessSecretVersionResponse struct {
	Payload struct {
		Data string `json:"data"`
	} `json:"payload"`
}

// googleAPIErrorResponse mirrors Google's standard JSON error envelope, so
// a failed request surfaces a real message (e.g. "PERMISSION_DENIED: ...")
// instead of just a bare HTTP status code.
type googleAPIErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// fetchAccessToken asks the host for a fresh, short-lived GCP access token
// immediately before every request. Tokens are never cached in this
// plugin's own state (it has none) and never baked into static config, so
// they can't go stale even though the host-side extism.Plugin instance
// wrapping this guest is itself cached for the life of the GDG process —
// see NewPluginLookupGSM on the host side.
func fetchAccessToken() (string, error) {
	offset := get_gcp_access_token()
	if offset == 0 {
		return "", errors.New("host did not return a GCP access token (see GDG host logs for the underlying cause)")
	}
	return pdk.ParamString(offset), nil
}

// Lookup resolves a Secret Manager resource name (e.g.
// "projects/<id>/secrets/<name>/versions/<n>", passed as this function's
// input) to its secret value by calling Secret Manager's
// AccessSecretVersion REST endpoint directly. Any ".json_field" suffix a
// caller used in a "lookup:gsm:<key>.<field>" reference is stripped off
// and applied host-side by the resolver (see
// internal/adapter/plugins/lookup/resolver.go) after this call returns —
// this plugin only ever sees the bare resource name.
//
//export Lookup
func Lookup() int32 {
	resourceName := pdk.InputString()
	if resourceName == "" {
		pdk.SetError(errors.New("gsm lookup plugin: no secret resource name provided"))
		return 1
	}

	token, err := fetchAccessToken()
	if err != nil {
		pdk.SetError(fmt.Errorf("gsm lookup plugin: %w", err))
		return 1
	}

	url := fmt.Sprintf("%s%s:access", secretManagerBaseURL, resourceName)
	req := pdk.NewHTTPRequest(pdk.MethodGet, url)
	req.SetHeader("Authorization", "Bearer "+token)
	resp := req.Send()

	body := resp.Body()

	if status := resp.Status(); status < 200 || status >= 300 {
		message := fmt.Sprintf("HTTP %d", status)
		var apiErr googleAPIErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			message = fmt.Sprintf("HTTP %d: %s (%s)", status, apiErr.Error.Message, apiErr.Error.Status)
		}
		pdk.SetError(fmt.Errorf("gsm lookup plugin: failed to access secret %q: %s", resourceName, message))
		return 1
	}

	var parsed accessSecretVersionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		pdk.SetError(fmt.Errorf("gsm lookup plugin: failed to parse Secret Manager response: %w", err))
		return 1
	}

	decoded, err := base64.StdEncoding.DecodeString(parsed.Payload.Data)
	if err != nil {
		pdk.SetError(fmt.Errorf("gsm lookup plugin: failed to base64-decode secret payload: %w", err))
		return 1
	}

	mem := pdk.AllocateBytes(decoded)
	pdk.OutputMemory(mem)
	return 0
}

func main() {}
