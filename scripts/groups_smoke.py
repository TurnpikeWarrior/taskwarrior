#!/usr/bin/env python3
"""Milestone 2 acceptance flow against the real binary in a truecolor Unix PTY."""
import os
from pathlib import Path
import tempfile

from terminal_smoke import Terminal, add, read, wait_file


def main():
    terminals = []
    with tempfile.TemporaryDirectory(prefix="taskwarrior-groups-") as tmp:
        root = Path(tmp)
        folder = root / "data"
        folder.mkdir()
        data, config = folder / "tasks.json", root / "config.toml"
        try:
            term = Terminal(data, config, no_color=False)
            terminals.append(term)
            term.expect("No tasks")
            doc = add(term, data, "agenda #MADA", 1)
            assert doc["tasks"][0]["text"] == "agenda"
            assert doc["groups"] == [{"id": "mada", "name": "MADA", "color_key": "blue"}]
            term.expect("38;2;38;139;255")
            add(term, data, "second", 2)
            term.send("g")
            term.send("\x1b[B")
            term.send("\r")
            wait_file(term, data, lambda d: d["tasks"][0].get("group_id") == "mada")
            term.send("c")
            term.send("\x1b[F")
            term.send("\r")
            wait_file(term, data, lambda d: d["tasks"][0].get("color_key") == "red")
            term.expect("38;2;255;64;64")
            term.send("g")
            term.send("\t")
            term.send("\r")
            term.send("\x1b[B")  # Current blue -> green.
            term.send("\r")
            wait_file(term, data, lambda d: d["groups"][0]["color_key"] == "green")
            assert read(data)["tasks"][0]["color_key"] == "red"
            term.send("\x1b")
            term.send("c")
            term.send("\x1b[H")
            term.send("\r")
            wait_file(term, data, lambda d: "color_key" not in d["tasks"][0])
            term.expect("38;2;0;214;107")
            term.send(" ")
            wait_file(term, data, lambda d: d["tasks"][0]["completed"])
            term.send("u")
            wait_file(term, data, lambda d: not d["tasks"][0]["completed"])
            # Create/assign a group and undo both together.
            before = read(data)
            term.send("g")
            term.send("\x1b[F")
            term.send("\r")
            term.send("\x1b[200~Team2\x1b[201~")
            term.resize(10, 2)
            term.expect("Resize")
            term.resize(60, 12)
            term.send("\r")
            wait_file(term, data, lambda d: len(d["groups"]) == 2)
            term.send("u")
            wait_file(term, data, lambda d: d == before)
            # Clicking menu items activates the same group color workflow.
            term.send("g")
            term.send("\x1b[<0;6;5M\x1b[<0;6;5m")  # Group color action.
            term.send("\x1b[<0;6;7M\x1b[<0;6;7m")  # Red, after Default + five entries.
            wait_file(term, data, lambda d: d["groups"][0]["color_key"] == "red")
            term.send("\x1b")
            term.send("u")
            wait_file(term, data, lambda d: d == before)
            # A failed composite group mutation must retain its draft and old file.
            old_bytes = data.read_bytes()
            os.chmod(folder, 0o500)
            term.send("g")
            term.send("\x1b[F")
            term.send("\r")
            term.send("\x1b[200~RetryGroup\x1b[201~")
            term.send("\r")
            term.expect("permission denied")
            assert data.read_bytes() == old_bytes
            os.chmod(folder, 0o700)
            term.send("\r")
            wait_file(term, data, lambda d: any(g["id"] == "retrygroup" for g in d["groups"]))
            term.send("u")
            wait_file(term, data, lambda d: d == before)
            term.send("c")
            term.send("\x1b[F")
            term.send("\r")
            wait_file(term, data, lambda d: d["tasks"][0].get("color_key") == "red")
            saved = data.read_bytes()
            term.send("q")
            term.close()
            restart = Terminal(data, config, no_color=False)
            terminals.append(restart)
            restart.expect("#MADA")
            restart.expect("38;2;255;64;64")
            restart.send("u")
            assert data.read_bytes() == saved
            restart.send("q")
            restart.close()
            assert not config.exists(), "Milestone 2 must not write TOML"
            assert not list(folder.glob(".taskwarrior-*"))
            print("PASS: groups, reuse, defaults, override/inherit, completion, nested color, mouse menus, composite undo, resize, write failure/retry, restart")
        finally:
            os.chmod(folder, 0o700)
            for term in terminals:
                term.cleanup()


if __name__ == "__main__":
    main()
