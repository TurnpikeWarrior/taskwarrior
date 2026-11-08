#!/usr/bin/env python3
"""Milestone 3 keyboard acceptance and TOML recovery against the real binary."""
import os
import json
from pathlib import Path
import re
import tempfile
import time
import tomllib
from terminal_smoke import Terminal, add, read, wait_file

ESC = "\x1b"
UP, DOWN, HOME, END = ESC+"[A", ESC+"[B", ESC+"[H", ESC+"[F"


def cfg(path):
    return tomllib.loads(path.read_text())


def wait_config(term, path, check):
    deadline = time.monotonic()+5
    while time.monotonic() < deadline:
        term.drain(0.03)
        if path.exists() and check(cfg(path)):
            return cfg(path)
    raise AssertionError(("config did not converge", term.transcript[-2000:]))


def palette_edit(term, name, color):
    term.send("t"+DOWN+"\r\r")
    term.send("\x15\x1b[200~"+name+"\x1b[201~\t\x15\x1b[200~"+color+"\x1b[201~\t\r")


def save_from_section(term):
    term.send(ESC)
    term.send(END+UP+"\r")


def main():
    terminals = []
    with tempfile.TemporaryDirectory(prefix="taskwarrior-style-") as tmp:
        root = Path(tmp)
        folder = root/"data"
        folder.mkdir()
        data, config = folder/"tasks.json", root/"config.toml"
        try:
            term = Terminal(data, config, no_color=False)
            terminals.append(term)
            add(term, data, "agenda #MADA", 1)
            original_json = data.read_bytes()
            term.send("t\r\r")  # Live numbered draft.
            term.send(ESC)
            term.send(ESC)
            assert not config.exists() and data.read_bytes() == original_json
            palette_edit(term, "Violet", "#aa55ff")
            term.expect("38;2;170;85;255")
            assert not config.exists()
            save_from_section(term)
            wait_config(term, config, lambda c: c["palette"]["entries"][0]["hex"] == "#AA55FF")
            assert data.read_bytes() == original_json
            term.send("t"+DOWN+"\r"+END+UP+UP+"\r")  # Remove selected Blue.
            save_from_section(term)
            wait_config(term, config, lambda c: len(c["palette"]["entries"]) == 4)
            assert read(data)["groups"][0]["color_key"] == "blue"
            term.send("g\t\r")
            term.expect("Missing: blue")
            term.send(ESC)
            term.send(ESC)
            term.send("t"+DOWN+"\r"+END+UP+"\r")  # Restore Defaults.
            save_from_section(term)
            wait_config(term, config, lambda c: len(c["palette"]["entries"]) == 5 and c["palette"]["entries"][0]["hex"] == "#268BFF")
            term.send("t"+DOWN+"\r"+END+UP*4+"\r")  # Add custom color.
            term.send("\x1b[200~Custom\x1b[201~\t\x15\x1b[200~#123abc\x1b[201~\t\r")
            save_from_section(term)
            doc = wait_config(term, config, lambda c: len(c["palette"]["entries"]) == 6)
            assert doc["palette"]["entries"][-1]["key"].startswith("custom-")
            assert doc["palette"]["entries"][-1]["hex"] == "#123ABC"
            # Keyboard-only appearance flow: numbered, comfortable, padding 2,
            # ASCII, border, footer, and mouse delete.
            term.send("t\r\r"+DOWN+"\r"+DOWN+"\r")
            term.send(HOME+DOWN*7+"\r"+DOWN+"\r"+DOWN+"\r"+DOWN*2+"\r")
            term.resize(10, 2)
            term.expect("Resize")
            term.resize(60, 12)
            save_from_section(term)
            doc = wait_config(term, config, lambda c: c["appearance"]["border"])
            app = doc["appearance"]
            assert (app["list_mode"], app["row_gap"], app["left_padding"], app["symbols"], app["footer"], app["mouse_delete"]) == ("numbered", 1, 2, "ascii", True, True)
            term.expect("[ ]")
            assert data.read_bytes() == original_json
            # Selected row delete control with border/numbering/padding offsets.
            term.send("\x1b[<0;27;2M\x1b[<0;27;2m")
            wait_file(term, data, lambda d: len(d["tasks"]) == 0)
            term.send("u")
            wait_file(term, data, lambda d: d == json.loads(original_json))
            # Real failed config replacement retains its editor draft.
            previous_config = config.read_bytes()
            term.send("t\r\r")  # numbered -> bulleted
            os.chmod(root, 0o500)
            save_from_section(term)
            term.expect("permission denied")
            assert config.read_bytes() == previous_config and data.read_bytes() == original_json
            os.chmod(root, 0o700)
            term.send("\r")  # Save remains focused after failure.
            wait_config(term, config, lambda c: c["appearance"]["list_mode"] == "bulleted")
            term.send(" ")
            wait_file(term, data, lambda d: d["tasks"][0]["completed"])
            completed_json = data.read_bytes()
            term.send("h")
            wait_config(term, config, lambda c: c["appearance"]["hide_completed"])
            term.expect("Show completed")
            assert data.read_bytes() == completed_json
            term.send("q")
            term.close()
            restart = Terminal(data, config, no_color=False)
            terminals.append(restart)
            restart.expect("Show completed")
            restart.send("h")
            wait_config(restart, config, lambda c: not c["appearance"]["hide_completed"])
            restart.send(" ")
            wait_file(restart, data, lambda d: not d["tasks"][0]["completed"])
            restart.send("q")
            restart.close()
            # Manual binding edits load on restart and survive an editor rewrite.
            text = config.read_text()
            for action, key in [("add", "n"), ("style", "s"), ("hide_completed", "v")]:
                text, count = re.subn(r"(?m)^"+action+r" = \[.*\]$", action+" = ['"+key+"']", text)
                assert count == 1, (action, text)
            config.write_text(text)
            custom = Terminal(data, config, no_color=False)
            terminals.append(custom)
            custom.expect("n add")
            custom.send("n")
            custom.expect("Add task")
            custom.send(ESC)
            custom.send("s")
            custom.send(END+UP+"\r")
            wait_config(custom, config, lambda c: c["keys"]["style"] == ["s"])
            custom.send("q")
            custom.close()
            bad = config.read_text().replace("version = 1", "version = 9", 1)
            config.write_text(bad)
            invalid = Terminal(data, config)
            terminals.append(invalid)
            invalid.expect("unsupported config version 9")
            invalid.close(expected=1)
            assert config.read_text() == bad
            print("PASS: draft cancel, palette edit/remove/restore/add, live preview, keyboard appearance, mouse delete, config failure/retry, hide/show, restart, bindings, invalid TOML")
        finally:
            os.chmod(root, 0o700)
            for term in terminals:
                term.cleanup()


if __name__ == "__main__":
    main()
