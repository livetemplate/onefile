# onefile

Generate one-file web apps for your terminal workflows. Each app runs only the commands you pass it.

Run one straight from GitHub with [tinkerdown](https://github.com/livetemplate/tinkerdown), passing your own command:

```sh
tinkerdown serve --allow-exec \
  github.com/livetemplate/onefile/logs \
  -- docker logs -f api
```

Open http://localhost:8080 to see the logs of your `api` container, live.

To generate a new one, ask your coding agent:

```
Read https://github.com/livetemplate/onefile/blob/main/AGENTS.md and build me a page over <your command>.
```

## Safe to share

A page only shows. Every command it runs is in the line you typed, so you can read a teammate's line before you run it.

- **A page from a URL can't run anything by itself.** It can't hold commands, actions, schedules, includes, file reads or unsafe HTML. `serve` refuses such a page before it starts ([the check](https://github.com/livetemplate/tinkerdown/blob/v0.9.1/cmd/tinkerdown/commands/remote_page.go#L242), [its tests](https://github.com/livetemplate/tinkerdown/blob/v0.9.1/cmd/tinkerdown/commands/remote_page_test.go#L183)).
- **Every command comes from you.** Commands come only from what you pass: the command after `--`, your `--source` and `--action` flags, or a `--config` file of your own. They run only with `--allow-exec`.
- **It stays on your machine.** `serve` listens on localhost unless you pass `--host` ([the default](https://github.com/livetemplate/tinkerdown/blob/v0.9.1/internal/config/config.go#L1025)).
- **You can pin the exact page.** After a fetch, `serve` prints the page's sha256. With `#sha256=<hex>` after the page's name, it refuses any other content ([the check](https://github.com/livetemplate/tinkerdown/blob/v0.9.1/cmd/tinkerdown/commands/remote_page.go#L223), [its test](https://github.com/livetemplate/tinkerdown/blob/v0.9.1/cmd/tinkerdown/commands/remote_page_test.go#L135)).

Two things are still yours to check. A page's name without a pin follows the repo's default branch, which can change after you read it, so share a pinned line. And a name can hold a look-alike owner, so check the owner before you run a line someone sent you.

## Why

LLMs make tools so cheap that most are used once and thrown away. onefile keeps that speed: an LLM can write you a page in minutes, and you can throw it away just as fast.

The difference is what happens when you want to keep one. A page captures a use case in its smallest form: what it takes in, the commands you pass as flags, and what it shows, one file of Markdown and HTML. Nothing else gets in.

So the workflow you repeat every week is easy to keep, rerun and share, and safe to run. And because the whole use case fits in one small file, an LLM can read it and remix it into the next one.

- **Your CLI does the work.** The page only shows what a tool you already trust prints.
- **Live, not a snapshot.** The page updates as your command prints, so you watch logs, tests and CI runs happen.
- **Lighter than the app it replaces.** No account, no workspace and no keys to learn.

## What it isn't

- No accounts and no hosted server: a page runs on your machine, at localhost.
- No stored data beyond what your tool keeps.
- Nothing to install besides tinkerdown and the tool itself.
- One file per app, never a project to set up.
- Not for watching production. It's for the work in front of you.

## Install

The apps need tinkerdown 0.9.1 or later:

```sh
brew install livetemplate/tap/tinkerdown
# or
go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@latest
```

## Apps

| App | What it does | Run it |
|-----|--------------|--------|
| [logs](logs/README.md) | Follows commands, containers and log files in one live timeline, with a tab per source, level and text filters, pause and clear. | `tinkerdown serve --allow-exec github.com/livetemplate/onefile/logs -- docker logs -f api` |
| [containers](containers/README.md) | Lists your Docker containers live, with start, stop, restart and kill buttons, and follows a container's logs when you click its name. | Takes its Docker commands as flags: copy the command from [its README](containers/README.md#run-it). |
| [ci](ci/README.md) | Your GitHub Actions runs as a live list: watch a run's jobs finish, read why it failed, rerun the failed jobs or cancel. Reads `gh run list`. | Takes its `gh` commands as flags: copy the command from [its README](ci/README.md#run-it). |
| [tests](tests/README.md) | Runs your tests on every save: one row per test, failures with their output, search by name. Reads `go test -json`. | From your Go module: `tinkerdown serve --allow-exec github.com/livetemplate/onefile/tests -- watchexec -e go -- go test -json ./...` |
| [http](http/README.md) | Lists the requests in your `.http` file, each with a Send button, and shows the response: status, headers, time, and the body indented when it's JSON. curl sends them. | Takes your `.http` file and a curl command as flags: copy the command from [its README](http/README.md#run-it). |
| [git](git/README.md) | Your repo's changed files, branches and log, kept current, with buttons to stage, unstage, switch branch and commit, and the diff of the file you click. Runs your `git`. | Run it in your repo with its `git` commands as flags: copy the command from [its README](git/README.md#run-it). |
| [db](db/README.md) | Lists the queries in your `.sql` file, each with a Run button, and shows the result as `sqlite3` or `psql` prints it. Read-only unless you change the command. | Takes your `.sql` file and a `sqlite3` or `psql` command as flags: copy the command from [its README](db/README.md#run-it). |

The apps planned next, most needed first, are in [ROADMAP.md](ROADMAP.md).

To see what an app takes, add `--help`: `tinkerdown serve github.com/livetemplate/onefile/http --help` lists its sources and actions. To run, build or remix an app with a coding agent, point it at [AGENTS.md](AGENTS.md).

## Share with your team

Send the whole line, pinned. Run it once yourself; `serve` prints the file the name stands for, and the pin:

```
Fetched github.com/livetemplate/onefile/logs (https://raw.githubusercontent.com/livetemplate/onefile/HEAD/logs/index.md) (sha256 3f2a…). Pin it with github.com/livetemplate/onefile/logs#sha256=3f2a…
```

Put the pinned name in the line you send. `@<commit>` after the name pins it too, as in `github.com/livetemplate/onefile/logs@30fc88d`. Your teammate sees every command it runs, and gets exactly the page you checked.

## Remix an app

Copy an app's page, change it, and serve your copy:

```sh
curl -O https://raw.githubusercontent.com/livetemplate/onefile/HEAD/logs/index.md
tinkerdown serve --allow-exec index.md -- docker logs -f api
```

Keep the commands in flags, as the apps do, so your page stays safe to share. Push it to your own repo, as `<app>/index.md`, and share it the same way by its name: `github.com/you/repo/<app>`. An agent can do this for you from [AGENTS.md](AGENTS.md#remix-an-app).

## Add an app to onefile

1. Make a folder named after the app, with the page as `index.md` and a `README.md` that says what it does and what it takes.
2. Build the page by following [AGENTS.md](AGENTS.md#build-an-app).
3. Add an end-to-end test as `e2e/<app>_test.go`. It can use the helpers in `e2e/helpers_test.go`, and its fixtures go in `e2e/testdata/<app>/`.
4. Add a row to the Apps table.

## Test

The end-to-end tests build tinkerdown from source, serve each app, and drive it in Chrome. They need Docker and Go:

```sh
cd e2e && go test ./...
```

`TINKERDOWN_SRC` points at the tinkerdown checkout to build. It defaults to `../../tinkerdown`, a sibling of this repo, and a relative path is read from `e2e/`. The build uses that checkout's committed client bundle, so run `make build-client` there after changing its client. To test a binary you already built, set `TINKERDOWN_BIN` instead.
