---
title: Logs
---

## [All] | [web] source = web | [worker] source = worker | [app] source = app

```lvt
<main lvt-source="all" class="viewer">
  <div class="toolbar">
    <form name="Filter">
      <select name="f.level" data-testid="level" lvt-on:change="Filter" aria-label="Level">
        <option value="">any level</option>
        <option value="error">error</option>
        <option value="warn">warn</option>
        <option value="info">info</option>
        <option value="debug">debug</option>
      </select>
      <input type="search" name="f.line" data-testid="search" placeholder="search" aria-label="Search"
             lvt-on:input="Filter" lvt-mod:debounce="250">
    </form>
    {{if .Paused}}<button name="Resume" data-testid="resume">Resume</button>{{else}}<button name="Pause" data-testid="pause">Pause</button>{{end}}
    <button name="Clear" data-testid="clear" class="secondary">Clear</button>
    <span class="status" data-testid="status">{{if .Paused}}paused{{else}}● live{{end}} · {{len .Data}}</span>
  </div>
  {{if .Error}}<p data-testid="error"><mark>{{.Error}}</mark></p>{{end}}

  <ol class="log" data-testid="log" lvt-fx:scroll="bottom-sticky">
    {{- range .Data}}<li data-key="{{.seq}}" class="lvl-{{.level}} stream-{{.stream}} src-{{.source}}"><time>{{.time}}</time><b class="src">{{.source}}</b><i class="lvl">{{.level}}</i><span class="msg">{{.msg}}</span><span class="std">{{.stream}}</span></li>{{end -}}
  </ol>
</main>
```

<style>
.viewer .toolbar { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; margin-bottom: .5rem; }
.viewer .toolbar form { display: flex; gap: .5rem; flex: 1 1 18rem; min-width: 0; margin: 0; }
.viewer .toolbar select, .viewer .toolbar input, .viewer .toolbar button { margin: 0; width: auto; }
.viewer .toolbar input[type=search] { flex: 1; min-width: 0; }
.viewer .status { font-size: .85rem; opacity: .7; white-space: nowrap; }
.log { height: 70vh; overflow-y: auto; margin: 0; padding: 0; list-style: none;
       font: .8rem/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; border: 1px solid var(--pico-muted-border-color, #ddd); border-radius: 6px; }
/* content-visibility skips layout and paint for rows scrolled out of view */
.log li { content-visibility: auto; contain-intrinsic-size: auto 1.6rem; display: grid; grid-template-columns: 7.5rem 4.5rem 3.5rem 1fr auto; gap: .5rem; padding: .1rem .5rem; border-bottom: 1px solid rgba(127,127,127,.12); }
.log .src { font-weight: 600; overflow: hidden; text-overflow: ellipsis; }
.log .lvl { font-style: normal; text-transform: uppercase; font-size: .7rem; }
.log .msg { word-break: break-word; }
.log .std { visibility: hidden; font-size: .7rem; color: #999; }
.log .stream-stderr .std, .log .stream-system .std { visibility: visible; }
.log .lvl-error, .log .lvl-fatal { color: #d93526; }
.log .lvl-warn { color: #b06500; }
.log .lvl-debug, .log .lvl-trace, .log .stream-system { color: #888; }
.log .stream-system { font-style: italic; }
@media (max-width: 600px) {
  .viewer .toolbar form { flex-basis: 100%; }
  .log li { grid-template-columns: auto auto 1fr; }
  .log .src { display: none; }
  .log .std { justify-self: end; }
  .log .msg { grid-column: 1 / -1; grid-row: 2; }
}
</style>
