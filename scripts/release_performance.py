#!/usr/bin/env python3
"""Measure process-cold PTY startup, idle resources, and real 10k-task navigation.

No filesystem caches are purged. All runs use a local temporary directory.
"""
import argparse
import errno
import fcntl
import json
import os
from pathlib import Path
import platform
import pty
import select
import statistics
import struct
import subprocess
import termios
import time


def fixture(count):
    tasks = []
    for i in range(count):
        task = {"id": f"00000000-0000-4000-8000-{i+1:012d}", "text": f"Task {i:05d}", "completed": i >= count//2,
                "created_at": "2026-11-07T20:00:00Z", "updated_at": "2026-11-07T20:00:00Z"}
        if task["completed"]:
            task["completion_seq"] = count-i
        tasks.append(task)
    return {"version": 1, "next_completion_seq": count//2+1, "next_group_seq": 1, "tasks": tasks, "groups": []}


class Process:
    def __init__(self, binary, path, config, first):
        start = time.perf_counter_ns()
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            fcntl.ioctl(0, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
            os.environ["TERM"] = "xterm-256color"
            os.environ["NO_COLOR"] = "1"
            os.execv(binary, [binary, "--data", str(path), "--config", str(config)])
        os.set_blocking(self.fd, False)
        self.alive = True
        try:
            self.read_until(first)
        except BaseException:
            self.close(force=True)
            raise
        self.startup_ms = (time.perf_counter_ns()-start)/1e6

    def read_until(self, sentinel, timeout=10):
        output = b""
        deadline = time.monotonic()+timeout
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
            output += chunk
            if sentinel in output:
                return output
        raise AssertionError(("frame not observed", sentinel, output[-1500:]))

    def move(self, keys, sentinel):
        start = time.perf_counter_ns()
        os.write(self.fd, keys)
        self.read_until(sentinel)
        return (time.perf_counter_ns()-start)/1e6

    def close(self, force=False):
        if not self.alive:
            return
        if force:
            os.kill(self.pid, 9)
        else:
            os.write(self.fd, b"q")
        deadline = time.monotonic()+5
        while time.monotonic() < deadline:
            ready, _, _ = select.select([self.fd], [], [], 0.01)
            if ready:
                try:
                    os.read(self.fd, 65536)
                except OSError:
                    pass
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.alive = False
                os.close(self.fd)
                if not force:
                    assert os.waitstatus_to_exitcode(status) == 0, status
                return
        os.kill(self.pid, 9)
        os.waitpid(self.pid, 0)
        os.close(self.fd)
        self.alive = False
        raise AssertionError("process did not exit")


def summary(values):
    ordered = sorted(values)
    return {"samples": len(values), "first_ms": values[0], "min_ms": min(values), "median_ms": statistics.median(values),
            "p95_ms": ordered[min(len(ordered)-1, int(len(ordered)*0.95))], "max_ms": max(values)}


def resources(pid):
    fields = subprocess.check_output(["ps", "-o", "rss=", "-o", "time=", "-p", str(pid)], text=True).split()
    parts = fields[1].split(":")
    seconds = 0
    for part in parts:
        seconds = seconds*60+float(part)
    return int(fields[0]), seconds


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="bin/taskwarrior")
    parser.add_argument("--output", required=True)
    parser.add_argument("--samples", type=int, default=25)
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    import tempfile
    result = {"machine": platform.platform(), "python": platform.python_version(), "binary": Path(binary).name,
              "conditions": "80x24 Unix PTY; xterm-256color; NO_COLOR=1; local filesystem; process-cold launches; OS file cache not purged; includes Python pty.fork/exec and first complete visible frame"}
    with tempfile.TemporaryDirectory(prefix="taskwarrior-perf-") as tmp:
        root = Path(tmp)
        for count in [0, 10000]:
            data, config = root/f"tasks-{count}.json", root/f"config-{count}.toml"
            if count:
                data.write_text(json.dumps(fixture(count)))
            first = b"Task 00023" if count else b"No tasks. Press a to add."
            timings = []
            for _ in range(args.samples):
                process = Process(binary, data, config, first)
                try:
                    timings.append(process.startup_ms)
                finally:
                    process.close()
            result[f"startup_{count}_tasks"] = summary(timings)
            result[f"startup_{count}_tasks"]["raw_ms"] = timings
            process = Process(binary, data, config, first)
            try:
                time.sleep(0.5)
                rss_before, cpu_before = resources(process.pid)
                start = time.perf_counter()
                time.sleep(5)
                rss_after, cpu_after = resources(process.pid)
                elapsed = time.perf_counter()-start
                result[f"idle_{count}_tasks"] = {"interval_s": elapsed, "rss_kib": rss_after,
                    "rss_before_kib": rss_before, "cpu_time_delta_s": cpu_after-cpu_before,
                    "cpu_percent_from_ps_time": 100*(cpu_after-cpu_before)/elapsed,
                    "cpu_clock_resolution_note": "ps cumulative CPU time; typically 0.01-second resolution on macOS"}
                if count:
                    end, home = [], []
                    for _ in range(25):
                        end.append(process.move(b"\x1b[F", b"Task 09999"))
                        home.append(process.move(b"\x1b[H", b"Task 00023"))
                    result["navigation_10000_end"] = summary(end)
                    result["navigation_10000_home"] = summary(home)
            finally:
                process.close()
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, indent=2)+"\n")
    print(json.dumps({key: value for key, value in result.items() if key != "binary"}, indent=2))


if __name__ == "__main__":
    main()
