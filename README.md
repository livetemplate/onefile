# Log Viewer

A live log viewer built with [tinkerdown](https://github.com/livetemplate/tinkerdown). It follows commands, docker containers and log files, and shows them in one timeline with a tab per source, level and text filters, pause, and clear.

![Desktop: the worker tab filtered to warnings](docs/desktop.png)

## Run it

```sh
tinkerdown serve --allow-exec .
```

Then open http://localhost:8080. The default sources are two demo processes (`web` and `worker`) and the file `logs/app.log`, so the page shows lines straight away. `--allow-exec` lets sources run commands. Without it, only file sources work.

Stream sources need tinkerdown with stream support: the `stream-sources` branch until it is released.

## Add your own logs

Sources live in `tinkerdown.yaml`. Each `stream` source follows one command or one file:

```yaml
sources:
  api:
    type: stream
    cmd: docker logs -f --timestamps --tail 200 api
  syslog:
    type: stream
    file: /var/log/syslog
```

Then add the new names in two places:

1. `inputs` of the `all` source in `tinkerdown.yaml`, so they join the timeline.
2. The tabs heading in `index.md`, for example `| [api] source = api`.

`tinkerdown.yaml` has commented examples for docker, journald and kubectl. All stream options are in [tinkerdown's stream source docs](https://github.com/livetemplate/tinkerdown/blob/main/docs/sources/stream.md).

## Use it

| To | Do |
|----|----|
| Watch everything | Stay on **All**. The list follows new lines. |
| Read back | Scroll up. The list stops following until you scroll back to the bottom. |
| Focus on one source | Pick its tab. |
| Find errors | Set the level to **error**. It keeps applying when you switch tabs. |
| Find text | Type in the search box. |
| Freeze the view | **Pause**. **Resume** catches up. |
| Start fresh | **Clear** hides the lines shown so far. |

Lines from stderr and the viewer's own notes (such as a restarted command) are marked at the right of each line.

## Test

The end-to-end test builds tinkerdown from source, serves a copy of this app, and drives it in Chrome. It needs Docker and Go.

```sh
TINKERDOWN_SRC=../tinkerdown go test ./...
```

`TINKERDOWN_SRC` defaults to `../tinkerdown`.
