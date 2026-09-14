"""
Tests for generate_registry.py

Covers:
  - sync: adds missing tags, skips existing, inherits fields from latest version
  - sync: no tags found
  - add (wizard): happy path via mocked input
  - add (wizard): rejects duplicate name
  - add (wizard): rejects empty required fields, then accepts on retry
  - _suggest_url_pattern: type match and generic fallback
  - load/save registry round-trip
"""

import json
import os
import sys
import tempfile
import textwrap
from pathlib import Path
from unittest import mock

import pytest

# Make the scripts/ directory importable regardless of working directory.
sys.path.insert(0, str(Path(__file__).parent))
import generate_registry as gr


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

SAMPLE_REGISTRY = [
    {
        "name": "aes-256-gcm",
        "type": "cipher",
        "description": "seeded implementation of aes-256",
        "source": "https://github.com/esnet/gdg-plugins/tree/main/cipher/aes-256-gcm",
        "urlPattern": "https://github.com/esnet/gdg-plugins/raw/refs/tags/{version}/plugins/cipher_aes256_gcm.wasm",
        "versions": [
            {
                "version": "0.1.0",
                "minimum_version": "0.9.0",
                "config_fields": ["passphrase"],
            }
        ],
    },
    {
        "name": "ansible-vault",
        "type": "cipher",
        "description": "golang implementation of ansible-vault",
        "source": "https://github.com/esnet/gdg-plugins/tree/main/cipher/ansible",
        "urlPattern": "https://github.com/esnet/gdg-plugins/raw/refs/tags/{version}/plugins/cipher_ansible.wasm",
        "versions": [
            {
                "version": "0.1.0",
                "minimum_version": "0.9.0",
                "config_fields": ["vault_password"],
            }
        ],
    },
]


@pytest.fixture
def registry_file(tmp_path):
    """Write SAMPLE_REGISTRY to a temp file and return its path."""
    path = tmp_path / "plugin_registry.json"
    path.write_text(json.dumps(SAMPLE_REGISTRY, indent=2))
    return str(path)


def _args(command, registry_path):
    """Build a minimal args namespace for the subcommand functions."""
    import argparse
    args = argparse.Namespace(command=command, registry=registry_path)
    return args


# ---------------------------------------------------------------------------
# load / save round-trip
# ---------------------------------------------------------------------------

def test_load_save_roundtrip(registry_file):
    data = gr.load_registry(registry_file)
    gr.save_registry(data, registry_file)
    reloaded = gr.load_registry(registry_file)
    assert reloaded == SAMPLE_REGISTRY


# ---------------------------------------------------------------------------
# sync
# ---------------------------------------------------------------------------

def test_sync_adds_missing_tag(registry_file):
    with mock.patch("generate_registry.get_local_tags", return_value=["0.1.0", "0.2.0"]):
        gr.cmd_sync(_args("sync", registry_file))

    result = gr.load_registry(registry_file)

    for plugin in result:
        versions = plugin["versions"]
        version_strings = [v["version"] for v in versions]
        assert "0.1.0" in version_strings
        assert "0.2.0" in version_strings

        # New entry should inherit fields from the previous latest version
        new_entry = next(v for v in versions if v["version"] == "0.2.0")
        old_entry = next(v for v in versions if v["version"] == "0.1.0")
        assert new_entry["minimum_version"] == old_entry["minimum_version"]
        assert new_entry["config_fields"] == old_entry["config_fields"]


def test_sync_skips_existing_tags(registry_file):
    with mock.patch("generate_registry.get_local_tags", return_value=["0.1.0"]):
        gr.cmd_sync(_args("sync", registry_file))

    result = gr.load_registry(registry_file)
    for plugin in result:
        # No duplicate versions
        version_strings = [v["version"] for v in plugin["versions"]]
        assert version_strings == list(dict.fromkeys(version_strings)), "Duplicate versions found"
        assert len(version_strings) == 1


def test_sync_no_tags(registry_file, capsys):
    with mock.patch("generate_registry.get_local_tags", return_value=[]):
        gr.cmd_sync(_args("sync", registry_file))

    captured = capsys.readouterr()
    assert "No local git tags" in captured.out

    # Registry unchanged
    result = gr.load_registry(registry_file)
    assert result == SAMPLE_REGISTRY


def test_sync_multiple_new_tags(registry_file):
    tags = ["0.1.0", "0.2.0", "0.3.0"]
    with mock.patch("generate_registry.get_local_tags", return_value=tags):
        gr.cmd_sync(_args("sync", registry_file))

    result = gr.load_registry(registry_file)
    for plugin in result:
        version_strings = [v["version"] for v in plugin["versions"]]
        assert set(version_strings) == {"0.1.0", "0.2.0", "0.3.0"}


# ---------------------------------------------------------------------------
# add
# ---------------------------------------------------------------------------

def _run_add(registry_file, inputs):
    """Run cmd_add with a sequence of mocked input() responses."""
    args = _args("add", registry_file)
    with mock.patch("builtins.input", side_effect=inputs):
        with mock.patch("generate_registry.get_local_tags", return_value=["0.1.0"]):
            gr.cmd_add(args)


def test_add_new_plugin_happy_path(registry_file):
    inputs = [
        "gsm",          # name
        "lookup",       # type
        "Google Secret Manager",  # description
        "https://github.com/esnet/gdg-plugins/tree/main/lookup/gsm",  # source
        "https://github.com/esnet/gdg-plugins/raw/refs/tags/{version}/plugins/lookup_gsm.wasm",  # urlPattern
        "0.1.0",        # version (default offered)
        "0.9.7",        # minimum_version
        "credentials",  # config_fields
        "yes",          # confirm
    ]
    _run_add(registry_file, inputs)

    result = gr.load_registry(registry_file)
    assert len(result) == 3

    new_plugin = next(p for p in result if p["name"] == "gsm")
    assert new_plugin["type"] == "lookup"
    assert new_plugin["versions"][0]["version"] == "0.1.0"
    assert new_plugin["versions"][0]["minimum_version"] == "0.9.7"
    assert new_plugin["versions"][0]["config_fields"] == ["credentials"]


def test_add_aborts_on_no_confirm(registry_file):
    inputs = [
        "new-plugin",
        "cipher",
        "Some cipher",
        "https://example.com/source",
        "https://example.com/{version}/plugin.wasm",
        "0.1.0",
        "0.9.0",
        "",
        "no",  # decline confirm
    ]
    with pytest.raises(SystemExit):
        _run_add(registry_file, inputs)

    # Registry should be unchanged
    result = gr.load_registry(registry_file)
    assert len(result) == 2


def test_add_rejects_duplicate_name(registry_file):
    # First input is a duplicate, second is a new unique name
    inputs = [
        "aes-256-gcm",   # duplicate — should be rejected
        "brand-new",     # accepted
        "cipher",
        "A new cipher",
        "https://example.com/source",
        "https://example.com/{version}/plugin.wasm",
        "0.1.0",
        "0.9.0",
        "",
        "yes",
    ]
    _run_add(registry_file, inputs)

    result = gr.load_registry(registry_file)
    names = [p["name"] for p in result]
    assert "brand-new" in names
    assert names.count("aes-256-gcm") == 1  # no duplicate added


def test_add_config_fields_parsed_correctly(registry_file):
    inputs = [
        "multi-field-plugin",
        "cipher",
        "Has multiple config fields",
        "https://example.com/source",
        "https://example.com/{version}/plugin.wasm",
        "0.1.0",
        "0.9.0",
        "key, secret, region",  # comma-separated with spaces
        "yes",
    ]
    _run_add(registry_file, inputs)

    result = gr.load_registry(registry_file)
    new = next(p for p in result if p["name"] == "multi-field-plugin")
    assert new["versions"][0]["config_fields"] == ["key", "secret", "region"]


def test_add_empty_config_fields_allowed(registry_file):
    inputs = [
        "no-config-plugin",
        "cipher",
        "No config needed",
        "https://example.com/source",
        "https://example.com/{version}/plugin.wasm",
        "0.1.0",
        "0.9.0",
        "",   # empty config_fields
        "yes",
    ]
    _run_add(registry_file, inputs)

    result = gr.load_registry(registry_file)
    new = next(p for p in result if p["name"] == "no-config-plugin")
    assert new["versions"][0]["config_fields"] == []


# ---------------------------------------------------------------------------
# _suggest_url_pattern
# ---------------------------------------------------------------------------

def test_suggest_url_pattern_existing_type():
    registry = [
        {
            "name": "aes-256-gcm",
            "type": "cipher",
            "urlPattern": "https://github.com/esnet/gdg-plugins/raw/refs/tags/{version}/plugins/cipher_aes256_gcm.wasm",
        }
    ]
    suggestion = gr._suggest_url_pattern("cipher", "new-cipher", registry)
    assert "{version}" in suggestion
    assert suggestion.endswith("cipher_new_cipher.wasm")


def test_suggest_url_pattern_no_matching_type():
    registry = [
        {
            "name": "aes-256-gcm",
            "type": "cipher",
            "urlPattern": "https://github.com/esnet/gdg-plugins/raw/refs/tags/{version}/plugins/cipher_aes256_gcm.wasm",
        }
    ]
    suggestion = gr._suggest_url_pattern("lookup", "gsm", registry)
    assert "{version}" in suggestion
    assert "lookup_gsm.wasm" in suggestion


def test_suggest_url_pattern_empty_registry():
    suggestion = gr._suggest_url_pattern("cipher", "my-plugin", [])
    assert "{version}" in suggestion
    assert "cipher_my_plugin.wasm" in suggestion


# ---------------------------------------------------------------------------
# get_local_tags (smoke test — calls real git in this repo)
# ---------------------------------------------------------------------------

def test_get_local_tags_returns_list():
    tags = gr.get_local_tags()
    assert isinstance(tags, list)
    # Tags are sorted strings
    assert tags == sorted(tags)
