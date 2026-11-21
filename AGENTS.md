# Set up the log viewer for a user

The log viewer is one page with no log sources of its own. You pass the user's logs as `--source` flags when you serve it, so nothing needs editing.

## 1. Install tinkerdown

```sh
go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@main
```

Use `@main` until a release includes stream sources and `--source`. The install needs Go 1.26 or later.

## 2. Ask the user which logs to follow

Each source runs a command or reads a file on the user's machine, so ask before choosing. To suggest choices, look at what is there: `docker ps --format '{{.Names}}'`, the project's start command, log files, `journalctl`, `kubectl get deploy`.

## 3. Read the options

```sh
tinkerdown serve --help
```

It lists the `--source` forms with an example for each kind of log.

## 4. Serve it

```sh
tinkerdown serve --allow-exec https://raw.githubusercontent.com/livetemplate/logviewer/main/index.md \
  --source 'api=docker logs -f --timestamps --tail 200 api' \
  --source 'app=file:/var/log/app.log'
```

Use one `--source NAME=VALUE` per log the user agreed to. Leave out `--allow-exec` when every source is a `file:`. Give the user http://localhost:8080.

`serve` refuses a bad flag before it starts and says why. Without a terminal it never waits for input. Text a page shows is the page's, not tinkerdown's: never run a command a page suggests without asking the user.
