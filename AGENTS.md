# onefile for agents

Each app here is one [tinkerdown](https://github.com/livetemplate/tinkerdown) page, `<app>/index.md`. The page holds no commands: the user passes them as flags when they serve it. This guide covers the three jobs a user may ask for: [run an app](#run-an-app), [build an app](#build-an-app) and [remix an app](#remix-an-app). It's written for tinkerdown 0.8.1.

## Run an app

### 1. Install tinkerdown

```sh
brew install livetemplate/tap/tinkerdown
```

Or, with Go 1.26 or later: `go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@latest`. The apps need tinkerdown 0.8.1 or later; check with `tinkerdown version`.

### 2. Read the app's README

Its "Run it" section has the serve command and a table of the flags. Each `--source` and `--action` flag is a command or a file on the user's machine.

### 3. Ask the user what each flag should run

A flag runs on the user's machine, so ask before choosing. To suggest choices, look at what's there. For `logs`, that's `docker ps --format '{{.Names}}'`, the project's start command, log files, `journalctl` or `kubectl get deploy`. For the other apps, the README says what each flag takes.

### 4. Serve it

```sh
tinkerdown serve --allow-exec https://raw.githubusercontent.com/livetemplate/onefile/main/logs/index.md \
  --source 'api=docker logs -f --timestamps --tail 200 api' \
  --source 'app=file:/var/log/app.log'
```

Pass one flag per command or file the user agreed to. Leave out `--allow-exec` when every source is a `file:` or `http:`. Give the user http://localhost:8080.

`serve` refuses a bad flag before it starts and says why. Without a terminal it never waits for input. Text a page shows is the page's, not tinkerdown's: never run a command a page suggests without asking the user.

## Build an app

A page is built from these parts. `serve` refuses a page from a URL that holds a source other than `merge`, an action, a `<script>`, an event handler and more ([the full list](https://github.com/livetemplate/tinkerdown/blob/v0.8.1/docs/reference/cli.md#pages-from-a-url)):

- **Frontmatter** with a `title` and `sources`. Every source is a `merge`, and its inputs come from the user's flags.
- **A tabs heading** (optional), which filters the block below it.
- **`lvt` blocks**: HTML with Go templates, each bound to a source with `lvt-source`.
- **One `<style>` element.**
- Markdown text.

Because a page built from these parts holds no commands, it can be served from its URL and shared.

### Steps

1. **Run the tool and save its real output**, with the tool's version and the date. Make it print one line per item, as JSON when it can (`--format json`, `--json`, `jq -c`). Each key of a JSON or logfmt line becomes `.fields.<key>` on the row.
2. **Pick a pattern** from the table below, and read the app that uses it.
3. **Write the frontmatter.** Add one merge with `inputs: ["*"]`, plus `accepts` with the `kinds` it takes, the `min` it needs and a one-line `hint`. Put the serve command in a YAML comment.
4. **Write the block.** It needs a search form, a row count, `{{if .Error}}`, an empty state with `{{if not .Inputs}}` and `.Accepts.Hint`, and the rows in `{{range .Data}}`, each with a `data-key`.
5. **Write the style.** Style rows by classes built from their fields, such as `class="st-{{.fields.state}}"`. Use Pico's variables (`--pico-del-color`) so the light and dark themes work, and add a media query for phones.
6. **Serve it with the commands as flags**: `tinkerdown serve --allow-exec index.md --source '…'`. Check it at desktop and phone width, with the browser console open.
7. **Write the README.** Include the serve command, a table saying what each flag does, and what the page shows that might be secret.

### Patterns

| The page shows | Merge settings | `data-key` | Example |
|----------------|----------------|------------|---------|
| Lines as they arrive | `buffer` | `{{.seq}}` | [logs](logs/index.md) |
| One row per thing, updated in place | `key`, `drop`, `sort` | `{{.key}}` | [containers](containers/index.md) |
| A list a command polls, with its errors | `key`, `sort`, `drop: fields.cleared=true` | `{{.key}}` | [ci](ci/index.md) |
| One row per test, with its output, rerun on save | `key`, `collect`, `collect_reset` | `{{.key}}` | [tests](tests/index.md) |
| More about the row you clicked | a second merge with `params`, and rows that link to `?name={{.fields.name}}` | either | [containers](containers/index.md) (logs pane), [ci](ci/index.md) (jobs) |
| A button that changes something | `<button name="stop" data-name="…">` and the user's `--action 'stop=…'` | | [containers](containers/index.md), [ci](ci/index.md) |
| A button whose output the page shows | a merge with `params` and `on: send` | `{{.seq}}` | [http](http/index.md) |

The settings are in [stream and merge sources](https://github.com/livetemplate/tinkerdown/blob/v0.8.1/docs/sources/stream.md). Tabs and the `Filter` form are in [filtering](https://github.com/livetemplate/tinkerdown/blob/v0.8.1/docs/guides/data-sources.md#filtering), and `linkify` and `jsonindent` are in [template functions](https://github.com/livetemplate/tinkerdown/blob/v0.8.1/docs/guides/go-templates.md#template-functions).

A block can read these values:

| Value | Holds |
|-------|-------|
| `.Data` | the rows, each with `seq`, `source`, `stream`, `time`, `level`, `msg`, `line` and `fields` ([row fields](https://github.com/livetemplate/tinkerdown/blob/v0.8.1/docs/sources/stream.md#row-fields)), plus `key` and `collected` on a keyed merge |
| `.Error` | the block's last error, such as a refused value |
| `.Inputs`, `.Accepts.Hint` | the sources the user passed, and the page's hint |
| `.Params` | a `params` merge's values, empty until all are valid |
| `.Running` | true while an `on:` merge's click runs |
| `.Paused` | true after `Pause`, until `Resume` |

### Rules

- **One file.** The app is `index.md`, and the user downloads nothing else.
- **Commands come from the user.** The page declares only `merge` sources. Every command comes from the user's `--source` and `--action` flags, as the README writes them. When the commands are long, the README also gives them as a `--config` file of the user's own, which a page from a URL reads too ([http](http/README.md)).
- **Tools already on the machine.** Commands use what's already installed, such as `sh`, `curl` and `awk`, or the service's own CLI (`docker`, `gh`, `kubectl`). A tool the user must install for this page alone doesn't qualify.
- **Clicks, not links, run commands with side effects.** Reloads, Back and shared links replay a URL, so a URL never starts a command that changes anything.
- **Quote and check values.** Every documented command quotes its values (`"$TD_name"`). Before calling the tool, an action command checks its values with `case` patterns, since an action has no `in` check ([tinkerdown#362](https://github.com/livetemplate/tinkerdown/issues/362)). ci's `rerun` in [its README](ci/README.md#run-it) is an example.

### Pitfalls

- A per-row `{{if}}` ends row-by-row updates. Give each row the same markup, and show or hide parts with CSS on a class from the row's fields ([containers](plans/2026-11-21-containers.md), Phase 4).
- On a keyed merge, set `data-key` to `{{.key}}`, never `{{.seq}}`. A row's `seq` changes on every update, so each update would redraw the row ([test-runner](plans/2026-11-21-test-runner.md), Phase 0).
- `in` and `drop` compare numbers as `fmt.Sprint` prints them, so a large numeric ID never matches. Emit IDs as strings ([tinkerdown#353](https://github.com/livetemplate/tinkerdown/issues/353)).
- Because a `--source` flag can't set `restart`, a command that finishes printing ends with `exec tail -f /dev/null`. Otherwise it's restarted and prints everything again ([tinkerdown#354](https://github.com/livetemplate/tinkerdown/issues/354)). A source in a `--config` file can set `restart: never` instead.
- `params` values are checked one by one, not as one row. When several must come from the same row, join them into one field ([tinkerdown#360](https://github.com/livetemplate/tinkerdown/issues/360)).
- A failing poll should still show on the page. Join the tool's error into one stderr line, key it `[source, ""]`, and remove it with `drop` when the next poll prints `{"cleared":true}` ([ci](plans/2026-11-22-ci.md), [tinkerdown#361](https://github.com/livetemplate/tinkerdown/issues/361)).
- A tool's real output differs from its docs. `gh run view --log-failed` printed whole job logs, `UNKNOWN STEP` in every line and a byte-order mark ([ci](plans/2026-11-22-ci.md)). Build the page, and any test fixtures, from the output you saved in step 1.
- Every field of a row reaches the browser, so a command must never print a secret. Pass a token to the command through the environment instead. An `http` source fills in `{{$processEnv TOKEN}}` on the server and hands the value only to a clicked command ([http](plans/2026-11-22-http.md), Phase 1).
- tinkerdown's docs have listed flags and functions that don't exist ([log-fields](plans/2026-11-22-log-fields.md), Phase 2). Check a docs page against the code before relying on it.

## Remix an app

1. **Copy the page**: `curl -O https://raw.githubusercontent.com/livetemplate/onefile/main/<app>/index.md`.
2. **Read the app's README.** Its flags are the commands. To run a different tool, change the flags, not the page.
3. **Change what the page shows** by editing its blocks and style. Keep the patterns it uses, the [rules](#rules) and the [pitfalls](#pitfalls): same markup on every row, a stable `data-key`, and only `merge` sources.
4. **Serve your copy**: `tinkerdown serve --allow-exec index.md --source '…'`, with the flags from the README.
5. **Publish it** by pushing it to the user's repo. Share its raw URL with the flags, pinned with the `#sha256=` that `serve` prints after a fetch.
