# jui

An interactive, terminal-UI wrapper around [`just`](https://github.com/casey/just).
Run `jui` in a project with a `justfile` to browse, fuzzy-filter, inspect, and
run recipes without memorizing their names.

## Requirements

- `just` on `PATH`
- Go 1.25+ (only needed to build)

## Install

```sh
go install .          # from this directory
# or
just install
```

This produces a `jui` binary (via `go install`, placed in `$GOBIN`/`$GOPATH/bin`).

## Usage

```sh
jui              # search upward from cwd for a justfile
jui -d path/to/project   # use a specific project directory
jui --version
```

Start typing to fuzzy-filter recipes by name/doc/group. The right-hand pane
shows the recipe's doc comment, parameters, dependencies, attributes
(`[group('db')]`, `[confirm(...)]`, etc.) and its source body.

Press **Enter** to run the highlighted recipe. If it takes parameters, jui
shows a small form (prefilled with defaults) before running — variadic
(`*args`/`+args`) fields are split on whitespace. jui then hands off to
`just` directly (via `exec`), so output, exit codes, and interactive
prompts (e.g. `[confirm]`) behave exactly as if you'd typed the command
yourself.

### Keybindings

| Key                | Action                                  |
|---------------------|------------------------------------------|
| type                | filter recipes                          |
| ↑ / ↓, ctrl+p/ctrl+n | move selection                          |
| Enter / Tab         | run recipe (or open the argument form)  |
| Ctrl+A              | toggle visibility of private (`_recipe`) recipes |
| Esc                 | clear filter, or quit if already empty  |
| Ctrl+C              | quit immediately                        |

In the argument form: **Tab** / **Shift+Tab** (or ↑/↓) move between fields,
**Enter** runs, **Esc** goes back to the list.

## How it works

jui shells out to `just --dump --dump-format json` to get structured recipe
data (docs, parameters, groups, attributes, dependencies, raw body), renders
the picker with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
and on selection replaces itself with `just <recipe> [args...]` via `exec`.
