# Set up the log viewer for a user

Follow these steps to install the log viewer and point it at the user's logs. The whole app is `index.md`: its frontmatter lists the log sources, and the page shows them in one timeline.

## 1. Install tinkerdown

```sh
go install github.com/livetemplate/tinkerdown/cmd/tinkerdown@main
```

Use `@main` until a release includes stream sources. Check with `tinkerdown --help`. The install needs Go 1.26 or later.

## 2. Ask which logs to follow

Ask the user. Don't guess, because each source runs a command or reads a file on their machine. To suggest choices, look at what is already there:

| Kind | How to find it | Source |
|------|----------------|--------|
| Docker container | `docker ps --format '{{.Names}}'` | `cmd: docker logs -f --timestamps --tail 200 NAME` |
| Web app the viewer should start | the project's start command, such as `npm run dev` or `go run ./cmd/server` | `cmd: cd PROJECT_DIR && START_COMMAND` |
| Log file | `/var/log/...`, or a path the app writes to | `file: /path/to/app.log` |
| systemd journal | `journalctl` exists | `cmd: journalctl -f -n 200 -o short-iso` |
| Kubernetes | `kubectl get deploy` | `cmd: kubectl logs -f --timestamps --tail 200 deploy/NAME` |

Run only commands the user agreed to. Paths and `cd` targets are relative to the log viewer's directory.

## 3. Edit `index.md`

Make three changes, and keep the names identical across them:

1. In the frontmatter `sources`, replace the demo sources (`web`, `worker`, `app`) with one `type: stream` source per log.
2. Set `inputs` of the `all` source to the new names.
3. Rewrite the tabs heading to match, for example `## [All] | [api] source = api | [db] source = db`.

All stream options, such as `format`, `restart` and `env`, are in [tinkerdown's stream source docs](https://github.com/livetemplate/tinkerdown/blob/main/docs/sources/stream.md).

## 4. Run it

```sh
tinkerdown serve --allow-exec .
```

`--allow-exec` lets `cmd` sources run. Leave it out when every source is a `file`. Give the user http://localhost:8080, or pass `--port` and `--host` to change where it listens.

A command that exits, or a file that doesn't exist yet, gets a `system` row in the list that says why. An invalid source, such as one with both `cmd` and `file`, stops the list from loading. `tinkerdown serve` then prints the reason after `Failed to create runtime state`. Fix the source in `index.md` and restart `tinkerdown serve`.
