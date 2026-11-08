#!/usr/bin/env python3
"""Build standalone release candidates and record SHA-256 fingerprints."""
import hashlib
import json
import os
from pathlib import Path
import subprocess


def main():
    output = Path("bin/release")
    output.mkdir(parents=True, exist_ok=True)
    manifest = {"go": subprocess.check_output(["go", "version"], text=True).strip(),
                "flags": ["-trimpath"], "cgo_enabled": False, "artifacts": [],
                "note": "Compilation evidence only; native runtime verification is recorded separately."}
    for system in ["darwin", "linux", "windows"]:
        for arch in ["amd64", "arm64"]:
            name = f"taskwarrior-{system}-{arch}" + (".exe" if system == "windows" else "")
            target = output/name
            env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="0")
            subprocess.run(["go", "build", "-trimpath", "-o", str(target), "./cmd/taskwarrior"], env=env, check=True)
            manifest["artifacts"].append({"os": system, "arch": arch, "path": str(target), "bytes": target.stat().st_size,
                                          "sha256": hashlib.sha256(target.read_bytes()).hexdigest()})
            print(f"PASS: {system}/{arch}", flush=True)
    evidence = Path("artifacts/release/builds.json")
    evidence.parent.mkdir(parents=True, exist_ok=True)
    evidence.write_text(json.dumps(manifest, indent=2)+"\n")
    Path("bin/release/SHA256SUMS").write_text("".join(f'{entry["sha256"]}  {Path(entry["path"]).name}\n' for entry in manifest["artifacts"]))


if __name__ == "__main__":
    main()
