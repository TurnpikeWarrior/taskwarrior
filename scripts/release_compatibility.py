#!/usr/bin/env python3
"""Check terminal protocols across color profiles and a private tmux server."""
import argparse
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile
import time
import terminal_smoke as smoke


def check_colors(raw, expected):
    sgr = re.findall(rb"\x1b\[([0-9;:]*)m", raw)
    params = [int(part) for code in sgr for part in code.replace(b":", b";").split(b";") if part]
    if expected == "truecolor":
        assert b"38;2;38;139;255" in raw, raw[-1500:]
    elif expected == "256":
        assert b"38;5;" in raw and b"38;2;" not in raw, raw[-1500:]
    elif expected == "16":
        assert any(30 <= value <= 37 or 90 <= value <= 97 for value in params), raw[-1500:]
        assert b"38;5;" not in raw and b"38;2;" not in raw, raw[-1500:]
    else:
        assert 38 not in params and 48 not in params and not any(30 <= value <= 37 or 90 <= value <= 107 for value in params), raw[-1500:]


def profile_case(root, name, environment, expected, config_text="", mouse=True):
    directory = root/name
    directory.mkdir()
    data, config = directory/"tasks.json", directory/"config.toml"
    if config_text:
        config.write_text(config_text)
    terminal = smoke.Terminal(data, config, no_color=False, environment=environment)
    try:
        terminal.expect("No tasks")
        smoke.add(terminal, data, "Profile task #MADA", 1)
        check_colors(terminal.transcript, expected)
        if "light" in name:
            assert b"48;2;255;255;255" in terminal.transcript
        if "dark" in name:
            assert b"48;2;0;0;0" in terminal.transcript
        if mouse:
            assert b"\x1b[?1002h" in terminal.transcript
            terminal.send("\x1b[<0;4;1M\x1b[<0;4;1m")
            smoke.wait_file(terminal, data, lambda d: d["tasks"][0]["completed"])
            terminal.send("u")
            smoke.wait_file(terminal, data, lambda d: not d["tasks"][0]["completed"])
        else:
            assert b"\x1b[?1002h" not in terminal.transcript
        terminal.send(" ")
        smoke.wait_file(terminal, data, lambda d: d["tasks"][0]["completed"])
        terminal.send(" ")
        smoke.wait_file(terminal, data, lambda d: not d["tasks"][0]["completed"])
        terminal.send("d")
        smoke.wait_file(terminal, data, lambda d: len(d["tasks"]) == 0)
        terminal.send("u")
        smoke.wait_file(terminal, data, lambda d: len(d["tasks"]) == 1)
        terminal.send("a")
        terminal.send("\x1b[200~retained draft\x1b[201~")
        terminal.resize(10, 2)
        terminal.expect("Resize")
        terminal.resize(80, 24)
        terminal.send("\x1b")
        assert len(smoke.read(data)["tasks"]) == 1
        terminal.send("a\x1b[200~save before Ctrl+C\x1b[201~\r\x03")
        terminal.close()
        assert len(smoke.read(data)["tasks"]) == 2
        assert b"\x1b[?1049l" in terminal.transcript and b"\x1b[?25h" in terminal.transcript
        if mouse:
            assert b"\x1b[?1002l" in terminal.transcript
        return {"name": name, "status": "pass", "environment": environment, "color_output": expected,
                "checks": ["add", "complete/reopen", "delete/undo", "SGR mouse" if mouse else "mouse disabled", "draft resize/cancel", "Ctrl+C save/cleanup"]}
    finally:
        terminal.cleanup()


def tmux_case(root, binary, output):
    tmux = shutil.which("tmux")
    if not tmux:
        return {"status": "unavailable", "reason": "tmux not installed"}
    socket = root/"tmux.sock"
    settings = root/"tmux.conf"
    settings.write_text('set -g status off\nset -g default-terminal "tmux-256color"\n')
    base = [tmux, "-S", str(socket), "-f", str(settings)]
    data, config = root/"tmux-tasks.json", root/"tmux-config.toml"
    def run(*args, check=True):
        return subprocess.run(base+list(args), text=True, capture_output=True, check=check)
    def pane():
        return run("capture-pane", "-p", "-t", "release:0.0").stdout
    def until(check, description):
        deadline = time.monotonic()+5
        while time.monotonic() < deadline:
            if check():
                return
            time.sleep(0.02)
        raise AssertionError((description, pane()))
    def literal(value):
        run("send-keys", "-t", "release:0.0", "-l", value)
    def key(value):
        run("send-keys", "-t", "release:0.0", value)
    command = shlex.join(["env", "-u", "NO_COLOR", "TERM=tmux-256color", "COLORTERM=truecolor", binary, "--data", str(data), "--config", str(config)])
    try:
        run("new-session", "-d", "-x", "80", "-y", "24", "-s", "release", command)
        until(lambda: "No tasks" in pane(), "empty tmux screen")
        for count, title in enumerate(["first", "second", "third"], 1):
            literal("a")
            until(lambda: "Add task" in pane(), "add prompt")
            literal(title)
            key("Enter")
            until(lambda: data.exists() and len(smoke.read(data)["tasks"]) == count, "tmux saved task")
        key("Down")
        literal(" ")
        until(lambda: smoke.read(data)["tasks"][1]["completed"], "complete in tmux")
        literal("u")
        until(lambda: not smoke.read(data)["tasks"][1]["completed"], "undo in tmux")
        literal("d")
        until(lambda: len(smoke.read(data)["tasks"]) == 2, "delete in tmux")
        literal("u")
        until(lambda: len(smoke.read(data)["tasks"]) == 3, "undo delete in tmux")
        literal("a")
        until(lambda: "Add task" in pane(), "resize prompt")
        literal("draft survives tmux resize")
        run("resize-window", "-t", "release:0", "-x", "10", "-y", "2")
        until(lambda: "Resize" in pane(), "tmux narrow hint")
        run("resize-window", "-t", "release:0", "-x", "80", "-y", "24")
        until(lambda: "draft survives tmux resize" in pane(), "tmux retained draft")
        key("Enter")
        until(lambda: len(smoke.read(data)["tasks"]) == 4, "tmux draft saved")
        until(lambda: "☐" in pane(), "tmux rendered checkbox")
        output.write_text(pane())
        literal("q")
        deadline = time.monotonic()+5
        while time.monotonic() < deadline:
            if run("has-session", "-t", "release", check=False).returncode != 0:
                break
            time.sleep(0.02)
        else:
            raise AssertionError("tmux process did not exit")
        assert len(smoke.read(data)["tasks"]) == 4
        return {"status": "pass", "version": subprocess.check_output([tmux, "-V"], text=True).strip(),
                "terminal": "tmux-256color", "checks": ["keyboard add/complete/delete/undo", "resize draft", "saved JSON", "clean session exit"],
                "capture": str(output)}
    finally:
        run("kill-server", check=False)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="bin/taskwarrior")
    parser.add_argument("--output", default="artifacts/release/terminal-compatibility.json")
    args = parser.parse_args()
    smoke.BINARY = str(Path(args.binary).resolve())
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    cases = [
        ("truecolor-dark", {"TERM": "xterm-256color", "COLORTERM": "truecolor", "NO_COLOR": None}, "truecolor", "[appearance]\nbackground='#000000'\nforeground='#FFFFFF'\n", True),
        ("truecolor-light", {"TERM": "xterm-256color", "COLORTERM": "truecolor", "NO_COLOR": None}, "truecolor", "[appearance]\nbackground='#FFFFFF'\nforeground='#000000'\n", True),
        ("ansi256", {"TERM": "xterm-256color", "COLORTERM": None, "NO_COLOR": None}, "256", "", True),
        ("ansi16", {"TERM": "xterm", "COLORTERM": None, "NO_COLOR": None}, "16", "", True),
        ("no-color-ascii", {"TERM": "xterm-256color", "COLORTERM": "truecolor", "NO_COLOR": "1"}, "none", "[appearance]\nsymbols='ascii'\n", True),
        ("dumb-mouse-disabled", {"TERM": "dumb", "COLORTERM": None, "NO_COLOR": "1"}, "none", "[appearance]\nsymbols='ascii'\nmouse=false\n", False),
    ]
    report = {"profiles": [], "limits": "protocol/output assertions in PTYs; no physical mouse, font, or perceived visual-contrast verification"}
    with tempfile.TemporaryDirectory(prefix="taskwarrior-release-") as tmp:
        root = Path(tmp)
        for case in cases:
            report["profiles"].append(profile_case(root, *case))
        report["tmux"] = tmux_case(root, smoke.BINARY, output.with_name("tmux-capture.txt"))
    output.write_text(json.dumps(report, indent=2)+"\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
