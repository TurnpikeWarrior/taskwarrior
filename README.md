# TaskWarrior TUI

A local, keyboard-first task manager with groups, colors, one-step undo, and
customizable layouts. Runs offline and saves changes automatically.

## Run

Requires Go 1.26+ and an interactive terminal.

```sh
go build -o bin/taskwarrior ./cmd/taskwarrior
./bin/taskwarrior
```

## Controls

| Key | Action |
| --- | --- |
| Up/k, Down/j | Select a task |
| Home, End | First/last task |
| a | Add a task; Enter saves, Esc cancels |
| Space | Complete or reopen |
| d | Delete selected task |
| u | Undo the last task, group, or color change |
| g | Assign a group or change its color |
| c | Set a task color or inherit its group color |
| t | Edit appearance and palette |
| h | Hide/show completed tasks |
| ? | Shortcut help |
| q, Esc | Quit the main list |
| Ctrl+C | Quit from any screen after pending saves |

Click a row to select it, its checkbox to toggle completion, and use the wheel
to scroll. Undo resets when the app closes.

## Groups and appearance

Add `Write meeting agenda #work` to create or reuse a group. Group names are
case-insensitive. Escape a final tag with `\#work` to keep it as literal text.

Press `t` to customize layout, spacing, colors, symbols, and the palette. Tab
moves focus, Enter activates a control, and Esc returns or cancels. Select Save
to keep editor changes. Set `NO_COLOR=1` to disable colors.

## Configuration

Tasks and settings are stored in your operating system's application-data and
configuration directories. Override their locations with `--data FILE` and
`--config FILE`. Edit keyboard bindings in TOML and restart to apply them.

Use `--help` for flags or `--version` for the installed version.
