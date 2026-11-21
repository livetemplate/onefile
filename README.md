# onefile

Small apps for developers, each one a single [tinkerdown](https://github.com/livetemplate/tinkerdown) page. Run any of them straight from GitHub, without cloning:

```sh
tinkerdown serve https://raw.githubusercontent.com/livetemplate/onefile/main/<app>/index.md
```

A page served from a URL can't run commands or read files on its own. An app that needs them, such as the log viewer, takes them as `--source` flags that you pass.

## Apps

| App | What it does | Run it |
|-----|--------------|--------|
| [logs](logs/README.md) | Follows commands, containers and log files in one live timeline, with a tab per source, level and text filters, pause and clear. | `tinkerdown serve --allow-exec https://raw.githubusercontent.com/livetemplate/onefile/main/logs/index.md --source 'api=docker logs -f api'` |

Setting an app up with a coding agent? Point it at [AGENTS.md](AGENTS.md).

The apps need tinkerdown built from `main` until the next release:

```sh
go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@main
```

## Add an app

1. Make a folder named after the app, with the page as `index.md` and a `README.md` that says what it does and what it takes.
2. Keep the page self-contained. A page served from a URL may only declare `merge` sources, and its sources come from `--source` flags.
3. Add an end-to-end test as `e2e/<app>_test.go`. It can use the helpers in `e2e/helpers_test.go`, and its fixtures go in `e2e/testdata/<app>/`.
4. Add a row to the table above.

## Test

The end-to-end tests build tinkerdown from source, serve each app, and drive it in Chrome. They need Docker and Go:

```sh
cd e2e && go test ./...
```

`TINKERDOWN_SRC` points at the tinkerdown checkout to build. It defaults to `../../tinkerdown`, a sibling of this repo, and a relative path is read from `e2e/`. The build uses that checkout's committed client bundle, so run `make build-client` there after changing its client. To test a binary you already built, set `TINKERDOWN_BIN` instead.
