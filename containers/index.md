---
title: Containers
# Pass the Docker commands when you serve this page; see README.md for the full command:
#   tinkerdown serve --allow-exec . --source 'docker=…' --source 'logs=…' --action 'stop=…' …
sources:
  containers:
    type: merge
    inputs: ["*"]
    buffer: 2000
    key: [fields.name]
    drop: fields.state=removed
    accepts:
      kinds: [cmd]
      min: 1
      hint: "A command that prints docker ps and then docker events as JSON lines; see the README"
  logview:
    type: merge
    inputs: [logs]
    buffer: 1000
    params:
      name: {in: containers.fields.name}
---

## [All] | [Running] fields.state = running | [Exited] fields.state = exited | [Paused] fields.state = paused

```lvt
<main lvt-source="containers">
  <form name="Filter" role="search">
    <input type="search" name="f.fields.name" data-testid="search" placeholder="container name"
           aria-label="Search containers" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <small data-testid="status">{{len .Data}} containers</small>
  {{if .Error}}<p data-testid="error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Inputs}}
    <p data-testid="empty">No feed yet. {{.Accepts.Hint}}.</p>
  {{end}}

  <ol class="ctr" data-testid="containers">
    {{range .Data}}
      <li data-key="{{.key}}" class="st-{{.fields.state}}">
        <small class="st">{{.fields.state}}</small>
        <a class="name" href="?name={{.fields.name}}">{{.fields.name}}</a>
        <span class="img">{{.fields.image}}</span>
        <small class="status">{{.fields.status}}</small>
        <span class="acts">
          <button name="start" data-name="{{.fields.name}}" class="a-start outline" aria-label="Start {{.fields.name}}">Start</button>
          <button name="stop" data-name="{{.fields.name}}" class="a-stop outline" aria-label="Stop {{.fields.name}}">Stop</button>
          <button name="restart" data-name="{{.fields.name}}" class="a-restart outline" aria-label="Restart {{.fields.name}}">Restart</button>
          <button name="kill" data-name="{{.fields.name}}" data-confirm="Kill {{.fields.name}}?" class="a-kill outline contrast" aria-label="Kill {{.fields.name}}">Kill</button>
        </span>
      </li>
    {{end}}
  </ol>
</main>
```

```lvt
<aside class="logpane" lvt-source="logview" aria-label="Logs">
  <header><b data-testid="log-title">{{with .Params.name}}{{.}} · {{end}}Logs</b> <small data-testid="log-status">{{len .Data}} lines</small></header>
  {{if .Error}}<p data-testid="log-error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Data}}<p class="hint" data-testid="log-hint">Click a container's name to follow its logs.</p>{{end}}
  <ol class="log" data-testid="logs" lvt-fx:scroll="bottom-sticky">
    {{range .Data}}<li data-key="{{.seq}}" class="stream-{{.stream}}"><time>{{.time}}</time> <span class="msg">{{.msg}}</span></li>{{end}}
  </ol>
</aside>
```

<style>
.ctr { padding: 0; list-style: none; font: .85rem/1.6 var(--pico-font-family-monospace); }
.ctr li { display: grid; grid-template-columns: 5.5rem minmax(0, 1fr) minmax(0, 12rem) minmax(0, 9rem) auto; gap: .5rem; align-items: center; }
.ctr :is(.name, .img, .status) { min-width: 0; overflow-wrap: anywhere; }
.ctr .name { font-weight: 600; }
.ctr .img { opacity: .75; }
.ctr .st::before { display: inline-block; width: 1.2em; content: "•"; }
.ctr .st-running { color: var(--pico-ins-color); }
.ctr .st-running .st::before { content: "●"; }
.ctr :is(.st-exited, .st-dead) { color: var(--pico-del-color); }
.ctr :is(.st-exited, .st-dead) .st::before { content: "✗"; }
.ctr .st-paused .st::before { content: "‖"; }
.ctr :is(.st-paused, .st-created) { color: var(--pico-muted-color); }

/* Only the buttons that fit the row's state, chosen by the row's class: a
   per-row {{if}} would make rows differ, and the list would re-render whole. */
.ctr .acts { display: flex; gap: .25rem; justify-content: flex-end; }
.ctr .acts button { display: none; padding: .1rem .45rem; font-size: .75rem; margin: 0; }
.ctr :is(.st-exited, .st-dead, .st-created) .a-start,
.ctr :is(.st-running, .st-restarting) :is(.a-stop, .a-restart, .a-kill),
.ctr .st-paused :is(.a-stop, .a-kill) { display: inline-block; }

.logpane header { display: flex; gap: .5rem; align-items: baseline; margin-bottom: .5rem; }
.logpane .hint { color: var(--pico-muted-color); margin: 0; }
.log { max-height: 50vh; overflow-y: auto; padding: 0; margin: 0; list-style: none; font: .8rem/1.5 var(--pico-font-family-monospace); }
.log li { margin: 0; overflow-wrap: anywhere; }
.log time { opacity: .6; }
.log .stream-stderr { color: var(--pico-del-color); }
.log .stream-system { opacity: .6; font-style: italic; }

/* Wide screens: the list on the left, the logs beside it and kept in view.
   Both blocks sit in the panel under the tabs heading. */
@media (min-width: 1000px) {
  .content-wrapper:has(.logpane) { max-width: none; }
  .tinkerdown-tabs-content:has(.logpane) { display: grid; grid-template-columns: minmax(0, 1fr) minmax(22rem, 42%); column-gap: 1.5rem; align-items: start; }
  /* The blocks' wide-screen bleed (negative margins) would overlap the columns. */
  .tinkerdown-tabs-content:has(.logpane) > .tinkerdown-interactive-block { margin: 1rem 0; max-width: none; }
  .tinkerdown-tabs-content > .tinkerdown-interactive-block:has(.logpane) { position: sticky; top: 1rem; }
  .tinkerdown-tabs-content > .tinkerdown-interactive-block:has(.logpane):hover { transform: none; }
  .logpane .log { max-height: calc(100vh - 17rem); } /* fits below the tabs */
  .tinkerdown-tabs-content:has(.logpane) .ctr li { grid-template-columns: 5.5rem minmax(10rem, 1fr) minmax(0, 9rem) auto; }
  .tinkerdown-tabs-content:has(.logpane) .ctr .img { display: none; }
}

/* Phones: the name and state on one line, the image and status under them,
   the buttons on their own line. */
@media (max-width: 700px) {
  .ctr li { grid-template-columns: 5rem minmax(0, 1fr); padding: .35rem 0; border-bottom: 1px solid var(--pico-muted-border-color); }
  .ctr :is(.img, .status) { grid-column: 1 / -1; }
  .ctr .acts { grid-column: 1 / -1; justify-content: flex-start; }
}
</style>
