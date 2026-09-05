## GDG Plugins

[![CI](https://github.com/esnet/gdg-plugins/actions/workflows/ci.yml/badge.svg)](https://github.com/esnet/gdg-plugins/actions/workflows/ci.yml)

This repo introduces a very alpha pattern which adds plugin support to [gdg](https://software.es.net/gdg/). 

There will eventually be several different types of plugins that can be incorporated. This also opens the door to 
allow the community to create their own plugins using their favorite language of choice.

Supported Types:

  - cipher: simple plugin that takes in a string as input and return a string as output. The plugin can define any additional 
configuration that will be passed on by GDG that the plugin will use. i.e. a `passphrase` for example.
  - lookup: resolves a `lookup:<provider>:<key>[.<json_field>]` reference (configured under `plugins.lookup` in `gdg.yml`) to a
secret value from an external store, e.g. Google Secret Manager. Like cipher plugins, the plugin's own instance is loaded once and
cached in memory for the life of the process; unlike cipher plugins, a resolved lookup *value* is also cached in memory so the
same reference is never re-resolved twice in one run. Any credentials the plugin needs (a GCP service account, for example) are
resolved host-side using the same `env:`/`file:` convention as every other plugin's config, and — where the backing API needs a
short-lived credential such as an OAuth2 access token — minted fresh by the host and handed to the guest via a host function on
every call, rather than baked into the plugin's static config.


The plugins in this repo are not intended to be a full comprehensive list. Feel free to implement your own. If you would like your 
work linked please add a PR and I will link to it in the README or create a .md doc for community contributions.

---

A few notes.

1. Plugin system is built on top of [extism](https://extism.org/). It supports a wide variety of languages to write your plugins in.
As long as the output is a .wasm file it should work, but you should use their SDK. There are several languages listed [here](https://extism.org/docs/quickstart/plugin-quickstart)
that you can choose from. 
2. If using golang, it should be highlighted that tinygo is needed to build the wasm file. Taskfile.yml has the flags I had to use to get the generated 
output to work, but your experience may vary. Once GDG 0.9.0 is out you can write your plugin and integrate it into GDG to test it out.


current plug-ins:
  - ansible-vault [wasm bin](https://raw.githubusercontent.com/esnet/gdg-plugins/refs/heads/main/plugins/cipher_ansible.wasm), [source code](https://github.com/esnet/gdg-plugins/tree/main/cipher/ansible) 
  - aes-256-gcm [wasm bin](https://raw.githubusercontent.com/esnet/gdg-plugins/refs/heads/main/plugins/cipher_aes256_gcm.wasm), [source code](https://github.com/esnet/gdg-plugins/tree/main/cipher/aes-256-gcm)
  - gsm (Google Secret Manager, lookup) [wasm bin](https://raw.githubusercontent.com/esnet/gdg-plugins/refs/heads/main/plugins/lookup_gsm.wasm), [source code](https://github.com/esnet/gdg-plugins/tree/main/lookup/gsm)


---
### Building & Testing

CI (`.github/workflows/ci.yml`) builds every guest plugin and runs the full Go test suite on every push and pull request to `main` — it installs Go (from `go.mod`'s `go` directive), TinyGo, and [Task](https://taskfile.dev/), then runs `task build_all` followed by `go test -v ./...`, so it exercises exactly the same commands described below.

This repo uses [Task](https://taskfile.dev/) (`Taskfile.yml`) to drive [tinygo](https://tinygo.org/) builds. Install both, then:

```sh
# build every plugin's .wasm into ./plugins/
task build_all

# build just the GSM lookup plugin
task lookup_gsm

# build one of the cipher plugins
task cipher_ansible
task cipher_aes

# build everything, then run the Go test suite (host-side tests load the
# freshly built .wasm files from ./plugins/, so build_all always runs first)
task run_tests
```

Each guest plugin's source lives in its own directory (`cipher/ansible`, `cipher/aes-256-gcm`, `lookup/gsm`) and is built
with the flags in `Taskfile.yml`'s `TINY_FLAGS` var (`-target wasi -no-debug -tags=purego -scheduler=none`) — tinygo's WASI target
is what makes the extism host functions (HTTP requests, config, and any custom host functions like GSM's
`get_gcp_access_token`) available to the guest. If you don't have `Task` installed, the equivalent raw command for any plugin is:

```sh
tinygo build -o plugins/<output-name>.wasm -target wasi -no-debug -tags=purego -scheduler=none <path>/plugin.go
```

Each plugin directory that needs host-side verification also has a `_test.go` file tagged `!tinygo` (so it's excluded from the
tinygo build itself and only compiles under the normal `go` toolchain). These load the compiled `.wasm` from `../../plugins/`
with `github.com/extism/go-sdk` and call the guest's exported functions directly — see `cipher/ansible/ansible_plugin_test.go`
for the simplest example, or `lookup/gsm/gsm_plugin_test.go` for one that also registers a fake host function (GSM's guest
calls back into the host for a GCP access token, so its tests must supply `get_gcp_access_token` themselves — real GDG supplies
the real one). One of the GSM tests reaches the real Secret Manager API (with an intentionally invalid token, to verify error
handling) and is skipped under `go test -short`.

---
### Community Managed Plugins

  - 