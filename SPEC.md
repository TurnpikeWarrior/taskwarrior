# TaskWarrior TUI specification

Version 1.0 · Implementation handoff

TaskWarrior is a local, keyboard-first todo list for macOS, Linux, and Windows. Launch it, add a task, check it off, or remove it. The default screen is a compact list with the visual restraint of a classic terminal utility and vivid, customizable color accents.

Build a single Go binary named `taskwarrior`. Use Bubble Tea v2 for the event loop, Lip Gloss v2 for rendering, and Bubbles v2 text input for prompts. Pin compatible versions in `go.mod` and commit `go.sum`. The installed binary works offline. Version 1 covers one local list, optional groups, and presentation preferences. Sync, accounts, due dates, recurring tasks, priorities, subtasks, and manual reordering are outside the release scope. [Bubble Tea v2 guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) · [Lip Gloss documentation](https://github.com/charmbracelet/lipgloss) · [Bubbles releases](https://github.com/charmbracelet/bubbles/releases)

## 1. Task behavior

| Action | Result |
| --- | --- |
| Add | `a` opens a single-line prompt. Enter commits a trimmed, nonempty title and selects the new task at the top of the active list. Esc cancels. |
| Complete | Space toggles the selected task. Completed tasks appear below active tasks in most-recently-completed order. |
| Reopen | Return the task to its original creation order within the active section. |
| Hide completed | `h` toggles a persistent preference. Hidden tasks remain in storage and reappear in the same order when shown. |
| Delete | `d` removes the task immediately with no confirmation. A transient status line advertises undo. |
| Undo | `u` reverses the most recent successful task mutation. Undo restores the task's ID, position, completion state, group, and override as applicable. |

Keep a single in-memory undo slot for add, delete, complete/reopen, group assignment, and task/group color assignment. Creating a group as part of add or assignment belongs to the same undo operation. A successful undo consumes the slot. Navigation, visibility, style, and palette changes leave the slot available. Restart clears it. A failed mutation leaves the prior undo slot intact.

Store tasks in immutable creation order, newest first. On completion assign the next monotonic completion sequence. Clear that sequence on reopen. The displayed order is active tasks followed by completed tasks sorted by descending completion sequence. Store selection by task ID. Keep that ID after reorder if it remains visible. If it disappears, select the task now at its previous visible index, clamped to the last row. An empty list has no selection. On startup select the first visible task. Empty states show the current Add shortcut, and an all-hidden list also shows the current Show Completed shortcut.

Allow duplicate titles. Reject empty titles after trimming and parsing. Convert line breaks and tabs from pasted titles to spaces. Reject remaining terminal control characters in user-entered and loaded text, including ESC and C0/C1 controls. Use a 500-code-point title limit and show validation errors in the prompt.

## 2. Keyboard and mouse

| Main-list default | Action |
| --- | --- |
| Up / `k`, Down / `j` | Move one visible task, stopping at each end |
| Home / End | Jump to the first or last visible task |
| `a`, Space, `d`, `u` | Add, toggle completion, delete, undo |
| `g`, `c` | Open group assignment or task color picker |
| `t`, `h`, `?` | Open style editor, toggle completed visibility, open shortcut help |
| `q`, Esc | Quit from the main list |
| Enter | No action |

Task actions requiring selection are inert when no row is selected. Use Up/Down and Home/End in choice lists, Tab/Shift+Tab across form controls, Enter to activate, and Esc to cancel or return one level. Show focus on every screen. Route keys to the active prompt or dialog before any main-list shortcut. Letters and Space enter text in text fields. Help and optional footer show the effective bindings. Esc remains available in every screen. Ctrl+C follows the quit path, including handling an outstanding save.

Use cell-motion mouse reporting when enabled. A left click on the checkbox selects and toggles the clicked task once. A left click on the body selects it. The optional row delete control appears only for the selected row when enabled in settings and deletes immediately when clicked. Click menu items to activate them and form fields to focus them. Wheel scrolls visible rows or the open menu. Keyboard navigation brings selection back into view after wheel scrolling. Ignore background row clicks while a modal is open. Rendering and hit testing share the same cell coordinates, including borders, spacing, and clipping. Spacing rows have no hit targets. Every mouse action has a keyboard equivalent.

## 3. Groups and palette

Each task belongs to zero or one group. Grouping supplies a visible suffix such as `#MADA` and a shared color. The main list keeps its usual order across groups.

- `Prepare meeting agenda #MADA` creates or reuses `MADA` and saves `Prepare meeting agenda` as the title. Parse only a final whitespace-delimited token matching `#[A-Za-z0-9][A-Za-z0-9_-]{0,31}`. Reject a bare valid group token as a missing title. Earlier hashtags remain in the title. A trailing escaped token such as `\#MADA` becomes literal title text with the backslash removed.
- `g` opens a menu with **Ungrouped**, existing groups in case-insensitive alphabetical order, and **New group**. Creating a group validates the same name grammar as the inline syntax. Reuse a case-insensitive match and preserve its original display capitalization. Assigning or clearing a group preserves the task override.
- The group picker has a **Group color** action for the highlighted existing group. Enter assigns the highlighted group. Tab moves between the choice list and the Group color action. Group color offers the current palette and **Default foreground**. Empty groups remain available after their last task is removed.
- Each new group gets the next current palette entry in creation order. Store a monotonic group sequence and use its zero-based value modulo current palette length at creation. An empty palette yields default foreground. Existing assignments remain keyed to the original entry.

| Stable key | Label | Default HEX |
| --- | --- | --- |
| `blue` | Blue | `#268BFF` |
| `green` | Green | `#00D66B` |
| `yellow` | Yellow | `#FFE135` |
| `orange` | Orange | `#FF8C00` |
| `red` | Red | `#FF4040` |

`c` opens a compact picker with **Inherit** followed by the current palette in configured order. Each entry shows its name, HEX, and a swatch when supported. Enter applies the selection. Manage palette entries in the style editor or TOML. Add and edit require a unique case-insensitive label and exactly six HEX digits after `#`. Normalize HEX to uppercase. Assign a unique, immutable key at creation. In-app keys for custom colors use a `custom-` prefix plus a generated identifier. Editing a label or HEX preserves the key and updates all references when saved. Remove any entry, including a default. An empty palette is valid. Restore Defaults replaces the draft palette with the five entries above, using their original keys.

For active text, resolve task override first, then group color, then theme foreground. A cleared override inherits the group color. If the first assigned key is absent from the palette, use theme foreground and keep the reference. This includes a deleted task override on a colored group. Show missing assignments as such when their picker opens. Reintroducing the same key restores its references. Completed text uses the theme's completed treatment regardless of either color assignment.

## 4. Rendering and style editor

Default to a plain list, zero blank rows between tasks, one cell of left padding, terminal background and foreground, and no persistent header, border, or footer. Use square corners when the border is enabled. Keep the terminal's fixed-width typography, aligned columns, and vivid task text. Use a dedicated `❯` selection column with a space on unselected rows. Highlight selection without replacing the task's foreground. Default completed styling combines a checked box, muted text, and strikethrough on the title.

```text
  ❯ ☐ Prepare meeting agenda  #MADA
    ☐ Buy groceries
    ☑ Update documentation
```

The example shows column order. Actual completed titles receive strikethrough when supported.

- Plain uses no list marker. Numbered uses right-aligned `1.`, `2.`, and so on across all visible tasks, including completed tasks. Bullet uses `•` before the checkbox. Each mode retains selection and checkbox columns.
- Density is 0, 1, or 2 blank rows between tasks, with Compact, Comfortable, and Spacious labels. Left padding is separately adjustable from 0 through 4 cells.
- Use one line per task. Clip by terminal cell width and grapheme boundary, with an ellipsis when space allows. Hide the optional delete control and then the group suffix when needed to give the title room. Keep focus and completion indicators legible. At sizes below 12 columns or 3 rows show a clipped resize hint, keep modal drafts, and suspend mouse hit targets until resized.
- Render only the visible rows, with a viewport that follows keyboard selection. Recompute layout on resize. Use the same layout rules for the live style preview.

`t` opens a compact style editor with **Appearance**, **Palette**, and **Save / Cancel** controls. Open with a draft of the current settings. Preview draft changes immediately against sample active, selected, and completed rows. Appearance includes list mode, density, padding, foreground, background, completed foreground, selection marker color, Unicode or ASCII symbols, optional border, optional footer, mouse reporting, optional mouse delete control, and completed visibility. Color fields accept `terminal` or a valid HEX value. Palette offers Add, Edit, Remove, and Restore Defaults. Tab navigates between sections and controls. Enter activates the focused choice, including Save. Esc in a nested form returns to the draft, and Esc at editor level cancels the entire draft. Save validates and atomically writes TOML before promoting the draft. A failed save keeps the editor and draft open with a visible error.

## 5. Files and recovery

Use the `taskwarrior-tui` directory name to isolate these files. On Linux use `$XDG_DATA_HOME/taskwarrior-tui/tasks.json` and `$XDG_CONFIG_HOME/taskwarrior-tui/config.toml`, with `~/.local/share` and `~/.config` fallbacks. On macOS use `~/Library/Application Support/taskwarrior-tui/` for both files. On Windows use `%LOCALAPPDATA%\taskwarrior-tui\tasks.json` and `%APPDATA%\taskwarrior-tui\config.toml`. Flags `--data <file>` and `--config <file>` override the defaults. Reject paths that resolve to the same file. Provide `--help` and `--version`. Require interactive terminal input and output for the TUI.

### Task JSON version 1

```json
{
  "version": 1,
  "next_completion_seq": 1,
  "next_group_seq": 1,
  "tasks": [
    {
      "id": "549e2091-d6d0-4297-af83-6bcc99e61251",
      "text": "Prepare meeting agenda",
      "completed": false,
      "group_id": "mada",
      "created_at": "2026-11-07T20:00:00Z",
      "updated_at": "2026-11-07T20:00:00Z"
    }
  ],
  "groups": [
    { "id": "mada", "name": "MADA", "color_key": "blue" }
  ]
}
```

Use random UUIDs for tasks and lowercase names as stable group IDs. Encode timestamps in UTC RFC 3339. Optional task fields are `group_id`, `color_key`, and `completion_seq`. Completed tasks require a positive unique completion sequence, and active tasks omit it. Groups may omit `color_key`. Validate unique IDs, group references, sequence bounds, required fields, and schema version. Missing palette keys are valid. A missing data file starts an empty list. Invalid or unsupported task data stops startup with a path and diagnostic and leaves the file intact.

### TOML version 1

```toml
version = 1

[appearance]
list_mode = "plain"             # plain, numbered, bulleted
row_gap = 0                     # 0–2
left_padding = 1                # 0–4
hide_completed = false
border = false
footer = false
symbols = "unicode"            # unicode, ascii
foreground = "terminal"
background = "terminal"
completed_foreground = "terminal"
selection_foreground = "terminal"
mouse = true
mouse_delete = false

[keys]
add = ["a"]
toggle = ["space"]

# If [palette] is absent, load the five defaults.
# If present, the ordered entries array defines the entire palette.
# [palette]
# entries = [
#   { key = "blue", name = "Blue", hex = "#268BFF" },
#   { key = "custom-example", name = "Violet", hex = "#AA55FF" },
# ]
```

A missing config or omitted fields use compiled defaults. An explicit `entries = []` is an empty palette. Key arrays replace the listed main-list action's defaults. Supported action IDs are `up`, `down`, `first`, `last`, `add`, `toggle`, `delete`, `undo`, `group`, `color`, `style`, `hide_completed`, `help`, and `quit`. Accept printable single-character keys and named keys recognized by the event adapter. Reject empty bindings, collisions, unknown actions, and reserved Esc, Ctrl+C, and Enter. Main-list bindings are editable through TOML. Save from the editor preserves effective bindings, and a TOML rewrite may reformat the file and drop comments. Load manual edits on restart. Invalid TOML, unsupported versions, or invalid values stop startup with a diagnostic and leave the file intact.

### Write contract

Take an exclusive OS-backed lock for each resolved file path using stable sidecar lock files held for the process lifetime. Acquire locks in a consistent path order. A second instance sharing either path exits with a clear message. Use restrictive permissions where the platform supports them.

Apply one mutation at a time. Update the list optimistically for immediate feedback, save the candidate snapshot, and temporarily gate further mutations while the write completes. If a write fails before replacement, roll back that candidate, selection, and undo changes and show a persistent error with the affected path. Preserve an add draft on failure. A user can repeat the action after correcting the cause. Keep navigation and rendering responsive. Defer quit until an outstanding write resolves. `h` writes only TOML. Task and group mutations write only JSON. Palette edits write only TOML, leaving JSON references intact.

For both files, write to a uniquely named temporary file in the target directory, flush and close it, and replace the target atomically using the platform's replace-existing primitive. On Unix, sync the parent directory where supported. A post-replacement sync failure reports uncertain crash durability and keeps the published snapshot active. Never delete the old file as a step toward replacing it. Clean up a temporary file after a handled failure. Test replacement on a local filesystem on each target OS.

## 6. Terminal compatibility

Use Bubble Tea's alternate screen and let it restore terminal state on normal exit. Use its terminal color profile to render truecolor or downsample to ANSI 256 and ANSI 16. Honor `NO_COLOR` by suppressing colors. In monochrome or when faint and strikethrough are unsupported, the checkbox and selection marker still communicate state. A configured completed foreground takes precedence over default faint styling. ASCII mode uses `>`, `[ ]`, `[x]`, `-`, and `...`, with ASCII borders. Color fidelity, glyph widths, faint text, and strikethrough depend on the emulator, font, and multiplexer. Font family and size are configured in the terminal emulator. Mouse reporting can be disabled for terminals without reliable support or when using native text selection. [Bubble Tea v2 guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) · [Lip Gloss color profiles](https://github.com/charmbracelet/lipgloss#colors)

## 7. Architecture

| Package | Responsibility |
| --- | --- |
| `cmd/taskwarrior` | Flags, path resolution, startup, lock lifetime, exit diagnostics |
| `internal/tasks` | Task and group models, validation, commands, order, and undo without terminal dependencies |
| `internal/app` | Bubble Tea modes, input routing, selection, viewport, save results, and dialogs |
| `internal/style` | Appearance defaults, palette resolution, display-cell layout, and Lip Gloss rendering |
| `internal/config` | Typed TOML defaults, load, validate, and encode |
| `internal/storage` | JSON codec, platform locks, atomic replacement, and filesystem errors |

The event loop owns mutable state. Save commands receive snapshots and return explicit result messages. The view consumes state and terminal dimensions, and shares layout calculations with mouse routing. Use stable IDs for task operations, derive visible rows when their inputs change, and avoid polling. Establish typed appearance defaults and a dedicated renderer in Milestone 1 so later settings can feed the same view.

## 8. Milestones and acceptance

| Milestone | Deliverable | Acceptance gate |
| --- | --- | --- |
| **1. Minimal core** | Runnable binary, compact plain renderer, keyboard navigation and help, add/complete/delete, one-step undo, mouse selection and checkbox, and atomic JSON storage | Start empty, add three tasks and see newest first, complete one and reopen it, delete and undo, then quit and reopen to the same saved task state. Repeat with mouse where supported. Inject a write failure and confirm the previous file and task state survive. Use compiled defaults in this milestone. |
| **2. Groups and color** | Inline parsing, group picker, automatic group colors, color overrides, and five default entries | Assign `#MADA` inline and reuse it via `g`. Verify override, inheritance, and muted completed rows. Verify group and override data survive restart. |
| **3. Configuration and style editor** | TOML, live preview, palette management, binding overrides, all list modes and densities, completed visibility, optional chrome and mouse delete | Edit and remove a referenced HEX color and verify rendering and fallback. Cancel a draft and verify it has no effect. Save a draft and verify it persists across restart. Complete the full editing flow by keyboard. |
| **4. Release checks** | Platform builds, terminal compatibility, and performance measurements | Run the release checks below on target platforms. Record the reference machine, terminal profiles, results, and any platform limitations. |

### Tests and release checks

- **Domain tests** cover newest-first insertion, completion order, reopening, hidden rows, stable selection, undo consumption and restoration, inline escapes and invalid suffixes, group reuse, and duplicate titles.
- **Style tests** cover all five HEX defaults, precedence and missing-key behavior, palette editing and restore, numbering across sections, gaps and alignment, monochrome, ASCII, wide and combining characters, and narrow layouts.
- **Event tests** drive keys and mouse through the Bubble Tea model. Verify modal key isolation, checkbox single-toggle, hit targets after resizing and scrolling, disabled mouse, focus after reorder, and drafts retained on failure.
- **Storage tests** use temporary directories. Verify JSON and TOML round trips, missing files, malformed and unsupported versions, same-file detection, second-instance rejection, failed replacement, post-replacement reporting, and restart behavior. Parse the TOML example as a fixture.
- **Manual smoke tests** cover keyboard-only use from add to undo and save, mouse parity, alternate-screen cleanup, terminal resize, light and dark backgrounds, and truecolor, reduced color, and monochrome. Test on macOS, Linux, and Windows, including one multiplexer session. Run `go test ./...`, `go test -race ./...` where supported, `go vet ./...`, and native build checks.
- **Performance checks** measure cold startup on an identified reference machine with a goal of under 50 ms, record idle CPU and memory, and exercise navigation with 10,000 tasks. Record the measurements with the test conditions.

## Codex starter prompt

> Read `SPEC.md` and implement Milestone 1 in this repository. First outline the implementation order, then build the runnable Go TUI with compact Lip Gloss rendering, keyboard and mouse input, one-step undo, and atomic JSON persistence. Use typed appearance defaults so later styling can reuse the renderer. Run the milestone tests and a terminal smoke test, then report results and any gaps against the acceptance criteria. Stop at the Milestone 1 gate.
