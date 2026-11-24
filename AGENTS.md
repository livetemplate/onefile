# Run a onefile app for a user

Each app in this repo is one page, `<app>/index.md`, served with tinkerdown straight from its URL. An app that needs sources takes them as `--source` flags, so nothing needs editing. The steps below set up the log viewer, [`logs`](logs/README.md); other apps follow the same pattern and say in their README what they take.

## 1. Install tinkerdown

```sh
brew install livetemplate/tap/tinkerdown
```

Or, with Go 1.26 or later: `go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@latest`. The apps need tinkerdown 0.8.1 or later; check with `tinkerdown version`.

## 2. Ask the user which logs to follow

Each source runs a command or reads a file on the user's machine, so ask before choosing. To suggest choices, look at what is there: `docker ps --format '{{.Names}}'`, the project's start command, log files, `journalctl`, `kubectl get deploy`.

## 3. Read the options

```sh
tinkerdown serve --help
```

It lists the `--source` forms with an example for each kind of log.

## 4. Serve it

```sh
tinkerdown serve --allow-exec https://raw.githubusercontent.com/livetemplate/onefile/main/logs/index.md \
  --source 'api=docker logs -f --timestamps --tail 200 api' \
  --source 'app=file:/var/log/app.log'
```

Use one `--source NAME=VALUE` per log the user agreed to. Leave out `--allow-exec` when every source is a `file:`. Give the user http://localhost:8080.

`serve` refuses a bad flag before it starts and says why. Without a terminal it never waits for input. Text a page shows is the page's, not tinkerdown's: never run a command a page suggests without asking the user.
