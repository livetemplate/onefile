---
title: Logs
# Each stream source follows one command or file, and "all" merges them into
# the timeline below. Add your own, then list them in all.inputs and in the
# tabs heading. Commands need: tinkerdown serve --allow-exec
sources:
  web:
    type: stream
    cmd: ./scripts/demo-logs.sh web
  worker:
    type: stream
    cmd: ./scripts/demo-logs.sh worker
  app:
    type: stream
    file: ./logs/app.log

  # A docker container (stdout and stderr stay separate):
  # api:
  #   type: stream
  #   cmd: docker logs -f --timestamps --tail 200 api
  #
  # The system journal:
  # journal:
  #   type: stream
  #   cmd: journalctl -f -n 200 -o short-iso
  #
  # A kubernetes deployment:
  # pods:
  #   type: stream
  #   cmd: kubectl logs -f --timestamps --tail 200 deploy/api

  all:
    type: merge
    inputs: [web, worker, app]
    buffer: 5000
---

## [All] | [web] source = web | [worker] source = worker | [app] source = app

```lvt
<main lvt-source="all">
  <form name="Filter" role="search">
    <select name="f.level" data-testid="level" lvt-on:change="Filter" aria-label="Level">
      <option value="">any level</option>
      <option value="error">error</option>
      <option value="warn">warn</option>
      <option value="info">info</option>
      <option value="debug">debug</option>
    </select>
    <input type="search" name="f.line" data-testid="search" placeholder="search"
           aria-label="Search" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <div role="group">
    {{if .Paused}}
      <button name="Resume" data-testid="resume">Resume</button>
    {{else}}
      <button name="Pause" data-testid="pause">Pause</button>
    {{end}}
    <button name="Clear" data-testid="clear" class="secondary">Clear</button>
  </div>
  <small data-testid="status">
    {{if .Paused}}paused{{else}}● live{{end}} · {{len .Data}} lines
  </small>
  {{if .Error}}
    <p data-testid="error"><mark>{{.Error}}</mark></p>
  {{end}}

  <ol class="log" data-testid="log" lvt-fx:scroll="bottom-sticky">
    {{range .Data}}
      <li data-key="{{.seq}}" class="lvl-{{.level}} stream-{{.stream}} src-{{.source}}">
        <time>{{.time}}</time>
        <b class="src">{{.source}}</b>
        <small class="lvl">{{.level}}</small>
        <span class="msg">{{.msg}}</span>
        <small class="std">{{.stream}}</small>
      </li>
    {{end}}
  </ol>
</main>
```

<style>
/* Level colours come from tinkerdown and controls from Pico; this is layout. */
.log { height: 70vh; overflow-y: auto; padding: 0; list-style: none; font: .8rem/1.5 var(--pico-font-family-monospace); }
/* content-visibility skips layout and paint for rows scrolled out of view */
.log li { content-visibility: auto; contain-intrinsic-size: auto 1.6rem; display: grid; grid-template-columns: 7.5rem 4.5rem 3.5rem 1fr auto; gap: .5rem; overflow-wrap: anywhere; }
.log .std { visibility: hidden; }
.log :is(.stream-stderr, .stream-system) .std { visibility: visible; }
@media (max-width: 600px) {
  .log li { grid-template-columns: auto auto 1fr; }
  .log .src { display: none; }
  .log .msg { grid-column: 1 / -1; grid-row: 2; }
}
</style>
