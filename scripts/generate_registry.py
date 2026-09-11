#!/usr/bin/env python3
"""
generate_registry.py — manage plugin_registry.json

Subcommands:
  sync   Read local git tags and add any missing version entries to every
         plugin, inheriting minimum_version and config_fields from the
         plugin's most recent version.
  add    Interactive wizard to append a brand-new plugin definition.
"""

import argparse
import json
import os
import subprocess
import sys

REGISTRY_PATH = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "plugin_registry.json",
)

PLUGIN_TYPES = ("cipher", "lookup")


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

def load_registry(path: str = REGISTRY_PATH) -> list:
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def save_registry(registry: list, path: str = REGISTRY_PATH) -> None:
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(registry, fh, indent=2)
        fh.write("\n")


def get_local_tags() -> list[str]:
    """Return sorted list of local git tags (version strings)."""
    result = subprocess.run(
        ["git", "tag", "-l"],
        capture_output=True,
        text=True,
        check=True,
    )
    tags = [t.strip() for t in result.stdout.splitlines() if t.strip()]
    return sorted(tags)


def prompt(label: str, default: str = "") -> str:
    """Prompt user for input, showing an optional default."""
    if default:
        display = f"{label} [{default}]: "
    else:
        display = f"{label}: "
    value = input(display).strip()
    return value if value else default


def prompt_choice(label: str, choices: tuple, default: str = "") -> str:
    """Prompt for one of a fixed set of choices, re-prompting on invalid input."""
    choices_str = "/".join(choices)
    while True:
        value = prompt(f"{label} ({choices_str})", default)
        if value in choices:
            return value
        print(f"  Invalid choice '{value}'. Please enter one of: {choices_str}")


# ---------------------------------------------------------------------------
# sync subcommand
# ---------------------------------------------------------------------------

def cmd_sync(args) -> None:
    registry = load_registry(args.registry)
    tags = get_local_tags()

    if not tags:
        print("No local git tags found. Nothing to sync.")
        return

    total_added = 0

    for plugin in registry:
        existing_versions = {v["version"] for v in plugin.get("versions", [])}
        latest = plugin["versions"][-1] if plugin.get("versions") else {}

        added = []
        for tag in tags:
            if tag not in existing_versions:
                new_entry = {
                    "version": tag,
                    "minimum_version": latest.get("minimum_version", ""),
                    "config_fields": list(latest.get("config_fields", [])),
                }
                plugin["versions"].append(new_entry)
                added.append(tag)

        if added:
            print(f"  {plugin['name']}: added versions {', '.join(added)}")
            total_added += len(added)
        else:
            print(f"  {plugin['name']}: already up to date")

    save_registry(registry, args.registry)

    if total_added:
        print(f"\nSynced {total_added} new version entr{'y' if total_added == 1 else 'ies'} into {args.registry}")
    else:
        print("\nRegistry is already up to date.")


# ---------------------------------------------------------------------------
# add subcommand
# ---------------------------------------------------------------------------

def cmd_add(args) -> None:
    registry = load_registry(args.registry)
    existing_names = {p["name"] for p in registry}

    print("\nAdding a new plugin. Press Enter to accept a default value.\n")

    # --- name ---
    while True:
        name = prompt("Plugin name")
        if not name:
            print("  Name cannot be empty.")
            continue
        if name in existing_names:
            print(f"  A plugin named '{name}' already exists in the registry.")
            continue
        break

    # --- type ---
    plugin_type = prompt_choice("Plugin type", PLUGIN_TYPES)

    # --- description ---
    while True:
        description = prompt("Description")
        if description:
            break
        print("  Description cannot be empty.")

    # --- source ---
    while True:
        source = prompt("Source URL (e.g. https://github.com/esnet/gdg-plugins/tree/main/<path>)")
        if source:
            break
        print("  Source URL cannot be empty.")

    # --- urlPattern ---
    # Suggest a pattern derived from the existing entries' urlPattern structure.
    suggested_pattern = _suggest_url_pattern(plugin_type, name, registry)
    while True:
        url_pattern = prompt("URL pattern (use {version} as placeholder)", suggested_pattern)
        if url_pattern:
            break
        print("  URL pattern cannot be empty.")
    if "{version}" not in url_pattern:
        print("  Warning: '{version}' placeholder not found in URL pattern.")

    # --- first version entry ---
    tags = get_local_tags()
    default_version = tags[-1] if tags else ""

    print("\nFirst version entry:")

    while True:
        version = prompt("Version", default_version)
        if version:
            break
        print("  Version cannot be empty.")

    while True:
        minimum_version = prompt("Minimum GDG version required")
        if minimum_version:
            break
        print("  Minimum version cannot be empty.")

    config_fields_raw = prompt("Config fields (comma-separated, e.g. passphrase,key)", "")
    config_fields = [f.strip() for f in config_fields_raw.split(",") if f.strip()] if config_fields_raw else []

    new_plugin = {
        "name": name,
        "type": plugin_type,
        "description": description,
        "source": source,
        "urlPattern": url_pattern,
        "versions": [
            {
                "version": version,
                "minimum_version": minimum_version,
                "config_fields": config_fields,
            }
        ],
    }

    print("\nNew plugin entry to be added:")
    print(json.dumps(new_plugin, indent=2))

    confirm = prompt("\nAdd this plugin? (yes/no)", "yes")
    if confirm.lower() not in ("yes", "y"):
        print("Aborted. No changes written.")
        sys.exit(0)

    registry.append(new_plugin)
    save_registry(registry, args.registry)
    print(f"\nPlugin '{name}' added to {args.registry}")


def _suggest_url_pattern(plugin_type: str, name: str, registry: list) -> str:
    """
    Derive a plausible urlPattern from existing entries of the same type,
    substituting the new plugin's type-prefixed name.
    """
    slug = f"{plugin_type}_{name.replace('-', '_')}"
    for entry in registry:
        if entry.get("type") == plugin_type and entry.get("urlPattern"):
            # Replace the wasm filename stem with the new slug
            pattern = entry["urlPattern"]
            # Pattern looks like: .../plugins/<old_slug>.wasm
            base = pattern.rsplit("/", 1)[0]
            return f"{base}/{slug}.wasm"
    # Generic fallback
    return f"https://github.com/esnet/gdg-plugins/raw/refs/tags/{{version}}/plugins/{slug}.wasm"


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Generate and update plugin_registry.json",
    )
    parser.add_argument(
        "--registry",
        default=REGISTRY_PATH,
        help="Path to plugin_registry.json (default: repo root)",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("sync", help="Sync missing git tag versions into the registry")
    sub.add_parser("add", help="Interactively add a new plugin definition")

    return parser


def main(argv=None) -> None:
    parser = build_parser()
    args = parser.parse_args(argv)

    if args.command == "sync":
        cmd_sync(args)
    elif args.command == "add":
        cmd_add(args)


if __name__ == "__main__":
    main()
