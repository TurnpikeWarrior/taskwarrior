# TaskWarrior TUI

A local Go task list implementing **Milestones 1–3** of [SPEC.md](SPEC.md), with
release verification summarized in [docs/verification.md](docs/verification.md).

Requires Go 1.26 or newer to build. The built binary works offline.

```sh
go build -o bin/taskwarrior ./cmd/taskwarrior
./bin/taskwarrior
```

Use `--data FILE` to choose the JSON file. The default is under
`~/Library/Application Support/taskwarrior-tui/` on macOS,
`$XDG_DATA_HOME/taskwarrior-tui/` (or `~/.local/share/`) on Linux, and
`%LOCALAPPDATA%\taskwarrior-tui\` on Windows. Both input and output must be
interactive terminals. `--help` and `--version` also work without a terminal.

| Key | Action |
| --- | --- |
| Up/k, Down/j | Select a task |
| Home, End | First/last task |
| a | Add; Enter saves, Esc cancels |
| Space | Complete/reopen selected task |
| d | Delete selected task immediately |
| u | Undo the last successful task/group/color change once |
| g | Assign/clear a group, create a group, or edit a group color |
| c | Task color override, or Inherit |
| t | Style editor |
| h | Hide/show completed tasks (saved preference) |
| ? | Shortcut help; arrows/wheel scroll, Esc returns |
| q, Esc | Quit the list |
| Ctrl+C | Quit from any screen after any outstanding save |

Click a task to select it, or its checkbox to select and toggle it. The wheel
scrolls the list or open picker. Keyboard navigation brings selection back into
view. Pickers support Up/Down, Home/End, Enter, and mouse activation. In the group
picker, Tab or Shift+Tab moves between the list and the highlighted group's
**Group color** action. Esc returns one level.

## Groups and colors

Add `Prepare meeting agenda #MADA` to create or reuse MADA. Only a final,
whitespace-delimited `#name` is parsed. Group names start with a letter or digit,
contain only letters, digits, `_` or `-`, and are at most 32 characters. A final
`\#MADA` is literal title text with the backslash removed. Bare `#MADA` is rejected
as a missing title. Reuse is case-insensitive and preserves original capitalization.

New groups cycle through Blue (`#268BFF`), Green (`#00D66B`), Yellow (`#FFE135`),
Orange (`#FF8C00`), and Red (`#FF4040`). Groups remain available when empty. A task
color override takes precedence over its group; Inherit clears the override.
Group assignment preserves any override. Group color also offers Default
foreground. Completed rows stay muted regardless of either assignment. `NO_COLOR`
suppresses colors while retaining names, HEX labels, group suffixes, and indicators.

Missing palette references remain stored and are identified in their color picker.
They use terminal foreground until that key is available again. Palette editing
and custom colors are available in the style editor.

## Persistence and recovery

Tasks, groups, and color assignments are saved after each mutation using a synced
temporary file and atomic replacement. Creating a group as part of adding or
assigning a task is one undo operation. Reapplying an unchanged assignment leaves
the undo slot available. Restart clears undo.

A failed replacement restores the previous task/group state and undo slot. Add
and new-group drafts remain available for retry; a failed picker save keeps the
picker and chosen value open. A directory-sync failure after replacement keeps
the published state and reports uncertain crash durability. Errors remain visible
until a successful save. Quitting with an unresolved save error prints its
diagnostic to stderr and returns a nonzero exit code.

Exclusive sidecar locks protect both resolved file paths for the process lifetime.
`--config FILE` selects the TOML file and participates in locking and same-file
checks. Missing configuration uses defaults. Invalid configuration stops startup
with a path diagnostic and leaves the file intact.

## Style editor and configuration

Press `t`, then select Appearance or Palette. Up/Down and Home/End navigate lists;
Tab/Shift+Tab move focus; Enter activates the focused control. Appearance choices
cycle on Enter. Color fields open a form and accept `terminal` or `#RRGGBB`.
Palette forms use Name and HEX fields, followed by Apply and Cancel. The preview
uses draft settings immediately. Esc from a nested form returns to the draft;
Esc at editor level cancels all changes. Select Save to atomically write TOML and
activate the draft. Failed saves keep the draft open for retry.

Appearance covers list mode, density, padding, foreground/background, completed
and marker colors, symbols, border, footer, mouse, row delete, and visibility.
Palette supports Add, Edit, Remove, and Restore Defaults. Editing preserves keys;
removing an entry leaves references intact and uses theme foreground until its key
returns. Empty palettes are valid.

The default config location is next to task JSON on macOS, under
`$XDG_CONFIG_HOME/taskwarrior-tui/config.toml` (or `~/.config/`) on Linux, and under
`%APPDATA%\taskwarrior-tui\config.toml` on Windows. Main-list bindings can be
edited in TOML and load on restart. For example:

```toml
version = 1

[keys]
add = ["n"]
style = ["s"]
```

Each listed array replaces that action's defaults. All effective bindings must
be distinct. Esc, Ctrl+C, and Enter are reserved. Saving from the style editor
preserves effective bindings but may reformat TOML and drop comments. `h` writes
only TOML; task/group/color assignments write only JSON. Editor changes and
visibility changes preserve task undo.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
python3 scripts/terminal_smoke.py
python3 scripts/groups_smoke.py
python3 scripts/style_smoke.py
```

The smoke scripts require a Unix PTY and a non-root user to inject permission
failures. They use isolated temporary directories and synthesized terminal mouse
reports. See [Verification](docs/verification.md) for results, measurements, and
known limits.


## Release checks

```sh
python3 scripts/release_compatibility.py
python3 scripts/release_performance.py --output artifacts/release/performance.json
python3 scripts/release_builds.py
```

Release candidates for macOS, Linux, and Windows (amd64/arm64) and their checksums
are generated under `bin/release/`. The current working binary is
`0.4.0-milestone4`. macOS and Linux ARM64 execution checks passed. Other targets
are cross-compiled and are not runtime-certified. Generated reports belong under
ignored `artifacts/release/`. See [Verification](docs/verification.md) for startup
measurements and visual-test limits.


## Repository hygiene

Commit source, tests, the spec, documentation, `go.mod`, and `go.sum`.
Executables, generated reports, profiles, Python caches, environment files, and
credential files are ignored. Generated release evidence belongs under
`artifacts/release/`, which is also ignored.

The default task and config files live outside the checkout. For custom paths
inside the checkout, use the ignored `.local/` directory to keep personal tasks
and preferences out of Git:

```sh
./bin/taskwarrior --data .local/tasks.json --config .local/config.toml
```

Do not force-add ignored files. Review the staged diff before committing; ignore
rules cannot detect credentials pasted into otherwise valid source files.
