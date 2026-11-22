---
title: Logs
# This page has no log sources of its own: pass them when you serve it.
#   tinkerdown serve --allow-exec . --source 'api=docker logs -f --timestamps api' --source 'app=file:/var/log/app.log'
# tinkerdown serve --help shows more examples.
sources:
  all:
    type: merge
    inputs: ["*"]
    buffer: 5000
    accepts:
      min: 1
      hint: "Logs to follow: a container, a dev server or a log file"
---

## [All] | [*] source

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
  {{if not .Inputs}}
    <p data-testid="empty">No log sources yet. {{.Accepts.Hint}}. Restart with
      <code>--source 'NAME=COMMAND'</code> or <code>--source 'NAME=file:PATH'</code>.</p>
  {{end}}

  <ol class="log" data-testid="log" lvt-fx:scroll="bottom-sticky">
    {{range .Data}}
      <li data-key="{{.seq}}" class="lvl-{{.level}} stream-{{.stream}} src-{{.source}}">
        <time>{{.time}}</time>
        <b class="src">{{.source}}</b>
        <small class="lvl">{{.level}}</small>
        <span class="text"><span class="msg">{{linkify .msg}}</span>
          <span class="fields">{{range $k, $v := .fields}}<span>{{$k}}={{linkify $v}}</span> {{end}}</span></span>
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
/* Each field after the message, as key=value, muted. The row's
   overflow-wrap: anywhere keeps a long URL from widening the page. */
.log .fields span { margin-left: .75rem; color: var(--pico-muted-color); }
.log :is(.stream-stderr, .stream-system) .std { visibility: visible; }
@media (max-width: 600px) {
  .log li { grid-template-columns: auto auto 1fr; }
  .log .src { display: none; }
  .log .text { grid-column: 1 / -1; grid-row: 2; }
  .log .fields span { display: block; margin-left: 0; }
}
</style>
