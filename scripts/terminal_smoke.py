#!/usr/bin/env python3
"""Exercise the real binary in an isolated Unix pseudo-terminal; no dependencies."""
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import sys
import tempfile
import termios
import time

BINARY = str(Path(sys.argv[1] if len(sys.argv) > 1 else "bin/taskwarrior").resolve())


class Terminal:
    def __init__(self, data, config, no_color=True, environment=None):
        self.pid, self.fd = pty.fork()
        self.transcript = b""
        self.exited = False
        if self.pid == 0:
            fcntl.ioctl(0, termios.TIOCSWINSZ, struct.pack("HHHH", 12, 60, 0, 0))
            os.environ["TERM"] = "xterm-256color"
            if no_color:
                os.environ["NO_COLOR"] = "1"
            else:
                os.environ.pop("NO_COLOR", None)
                os.environ["COLORTERM"] = "truecolor"
            for key, value in (environment or {}).items():
                if value is None:
                    os.environ.pop(key, None)
                else:
                    os.environ[key] = value
            os.execv(BINARY, [BINARY, "--data", str(data), "--config", str(config)])
        os.set_blocking(self.fd, False)
        self.drain(0.3)

    def drain(self, duration=0.12):
        deadline = time.monotonic() + duration
        while time.monotonic() < deadline:
            ready, _, _ = select.select([self.fd], [], [], max(0, deadline-time.monotonic()))
            if not ready:
                break
            try:
                chunk = os.read(self.fd, 65536)
            except OSError as exc:
                if exc.errno == errno.EIO:
                    break
                raise
            if not chunk:
                break
            self.transcript += chunk

    def send(self, data):
        os.write(self.fd, data.encode() if isinstance(data, str) else data)
        self.drain()

    def expect(self, text):
        deadline = time.monotonic() + 5
        while text.encode() not in self.transcript and time.monotonic() < deadline:
            self.drain(0.05)
        assert text.encode() in self.transcript, (text, self.transcript[-2000:])

    def resize(self, width, height):
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        os.kill(self.pid, signal.SIGWINCH)
        self.drain()

    def close(self, expected=0):
        deadline = time.monotonic()+5
        while time.monotonic() < deadline:
            self.drain(0.05)
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.exited = True
                os.close(self.fd)
                assert os.waitstatus_to_exitcode(status) == expected, self.transcript[-2000:]
                return
        raise AssertionError("binary did not exit")

    def cleanup(self):
        if not self.exited:
            os.kill(self.pid, signal.SIGKILL)
            os.waitpid(self.pid, 0)
            os.close(self.fd)
            self.exited = True


def read(path):
    return json.loads(path.read_text())


def wait_file(terminal, path, check):
    deadline = time.monotonic()+5
    while time.monotonic() < deadline:
        terminal.drain(0.03)
        if path.exists() and check(read(path)):
            return read(path)
    raise AssertionError(("saved state did not converge", terminal.transcript[-2000:]))


def add(terminal, path, title, count):
    terminal.send("a")
    terminal.send("\x1b[200~"+title+"\x1b[201~")
    terminal.send("\r")
    return wait_file(terminal, path, lambda d: len(d["tasks"]) == count)


def main():
    terminals = []
    with tempfile.TemporaryDirectory(prefix="taskwarrior-smoke-") as tmp:
        root = Path(tmp)
        folder = root/"data"
        folder.mkdir()
        data, config = folder/"tasks.json", root/"config.toml"
        try:
            terminal = Terminal(data, config)
            terminals.append(terminal)
            terminal.expect("No tasks. Press a to add.")
            assert b"\x1b[?1049h" in terminal.transcript
            assert b"\x1b[?1002h" in terminal.transcript
            for i, title in enumerate(["first", "second", "third"], 1):
                doc = add(terminal, data, title, i)
            assert [t["text"] for t in doc["tasks"]] == ["third", "second", "first"]
            terminal.send("\x1b[B")
            terminal.send(" ")
            doc = wait_file(terminal, data, lambda d: d["tasks"][1]["completed"])
            assert doc["tasks"][1]["completion_seq"] == 1
            terminal.send(" ")
            doc = wait_file(terminal, data, lambda d: not d["tasks"][1]["completed"])
            before_delete = doc
            terminal.send("d")
            wait_file(terminal, data, lambda d: len(d["tasks"]) == 2)
            terminal.send("u")
            doc = wait_file(terminal, data, lambda d: len(d["tasks"]) == 3)
            assert doc == before_delete
            terminal.send("?")
            terminal.expect("Shortcut help")
            terminal.send("d")
            assert read(data) == doc
            terminal.send("\x1b")
            terminal.send("\x1b[H")
            # SGR mouse protocol coordinates are 1-based. Checkbox is x=4.
            terminal.send("\x1b[<0;4;1M\x1b[<0;4;1m")
            wait_file(terminal, data, lambda d: d["tasks"][0]["completed"])
            assert sum(t["completed"] for t in read(data)["tasks"]) == 1
            terminal.send("u")
            wait_file(terminal, data, lambda d: not d["tasks"][0]["completed"])
            terminal.send("\x1b[<0;8;2M\x1b[<0;8;2m")
            terminal.send(" ")
            wait_file(terminal, data, lambda d: d["tasks"][1]["completed"])
            terminal.send("u")
            wait_file(terminal, data, lambda d: not d["tasks"][1]["completed"])
            terminal.send("a")
            terminal.send("\x1b[200~draft survives resize\x1b[201~")
            terminal.resize(10, 2)
            terminal.expect("Resize")
            terminal.resize(60, 12)
            terminal.send("\r")
            wait_file(terminal, data, lambda d: len(d["tasks"]) == 4)
            # Fail the actual filesystem write while retaining the old target.
            old_bytes = data.read_bytes()
            os.chmod(folder, 0o500)
            terminal.send("a")
            terminal.send("\x1b[200~retry draft\x1b[201~")
            terminal.send("\r")
            terminal.expect("permission denied")
            assert data.read_bytes() == old_bytes
            os.chmod(folder, 0o700)
            terminal.send("\r")
            doc = wait_file(terminal, data, lambda d: len(d["tasks"]) == 5)
            assert doc["tasks"][0]["text"] == "retry draft"
            assert not list(folder.glob(".taskwarrior-*"))
            # A second real process must fail while either path is locked.
            second = Terminal(data, config)
            terminals.append(second)
            second.expect("another instance")
            second.close(expected=1)
            saved = data.read_bytes()
            terminal.send("q")
            terminal.close()
            assert b"\x1b[?1049l" in terminal.transcript
            assert b"\x1b[?25h" in terminal.transcript
            restarted = Terminal(data, config)
            terminals.append(restarted)
            restarted.expect("retry draft")
            restarted.send("u")  # Undo is cleared by restart.
            assert data.read_bytes() == saved
            restarted.send("q")
            restarted.close()
            print("PASS: PTY keyboard, mouse, help isolation, resize draft, failed-write retry, locks, restart, terminal cleanup")
        finally:
            os.chmod(folder, 0o700)
            for terminal in terminals:
                terminal.cleanup()


if __name__ == "__main__":
    main()
