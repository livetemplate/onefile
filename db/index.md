---
title: DB
# Pass your .sql file and your database's client when you serve this page; see README.md for both commands:
#   tinkerdown serve --allow-exec github.com/livetemplate/onefile/db \
#     --source sql:./queries.sql \
#     --source 'run=sqlite3 -safe -readonly -box ./app.db " $TD_sql"'
#   tinkerdown serve --allow-exec github.com/livetemplate/onefile/db \
#     --source sql:./queries.sql \
#     --source 'run=PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=30s" psql -X -v ON_ERROR_STOP=1 -c " $TD_sql"'
sources:
  list:
    type: merge
    inputs: ["*"]
    key: [fields.entry]
    accepts:
      kinds: [sql]
      min: 1
      hint: "--source sql:./queries.sql; see the README"
  result:
    type: merge
    inputs: [run]
    on: execute
    buffer: 5000
    line_max: 1MiB
    params:
      entry: {in: list.fields.entry}
---

# DB

```lvt
<main class="qpane" lvt-source="list">
  <form name="Filter" role="search">
    <input type="search" name="f.fields.text" placeholder="name or SQL"
           aria-label="Search queries" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <small>{{len .Data}} queries</small>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Inputs}}
    <p>No queries yet. {{.Accepts.Hint}}.</p>
  {{end}}

  <ol class="queries">
    {{range .Data}}
      <li data-key="{{.key}}" class="kind-{{.fields.kind}}">
        <b class="name">{{.fields.name}}</b>
        <code class="kind">{{.fields.kind}}</code>
        <button name="execute" data-entry="{{.fields.entry}}" class="outline"
                aria-label="Run {{.fields.name}}">Run</button>
        <span class="first">{{.line}}</span>
        <details class="sql"><summary>SQL · {{.fields.file}}:{{.fields.line}}</summary><pre>{{.fields.sql}}</pre></details>
      </li>
    {{end}}
  </ol>
</main>
```

```lvt
<aside class="result" lvt-source="result" aria-label="Result">
  <header>
    <b>Result</b>
    <small class="which">{{.Params.entry}}</small>
    <small aria-busy="{{.Running}}">{{if .Running}}● running{{else if .Data}}{{len .Data}} lines{{end}}</small>
  </header>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Params}}
    <p class="hint">Press Run on a query.</p>
  {{else if and (not .Running) (not .Data)}}
    <p class="hint">No rows</p>
  {{end}}
  <div class="scroll">
    <ol class="out">
      {{range .Data}}
        <li data-key="{{.seq}}" class="s-{{.stream}}"><i class="tag">{{.stream}}</i><span class="line">{{.line}}</span></li>
      {{end}}
    </ol>
  </div>
</aside>
```

<style>
.queries { padding: 0; list-style: none; }
.queries li { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: .1rem .75rem; align-items: center;
  padding: .4rem 0; border-bottom: 1px solid var(--pico-muted-border-color); }
.queries .name { min-width: 0; overflow-wrap: anywhere; }
.queries .kind { font-size: .75rem; padding: .05rem .35rem; }
.queries button { padding: .1rem .6rem; font-size: .8rem; margin: 0; }
.queries .first { grid-column: 1 / -1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  color: var(--pico-muted-color); font: .8rem/1.5 var(--pico-font-family-monospace); }
.queries .sql { grid-column: 1 / -1; margin: 0; font-size: .8rem; }
.queries .sql summary { color: var(--pico-muted-color); }
.queries .sql pre { margin: .25rem 0 0; padding: .5rem; white-space: pre-wrap; overflow-wrap: anywhere; }
/* A query that may write: every kind but the reads is marked, so a keyword
   missing here, such as SET, is marked too. The read-only connection refuses
   a write. */
.queries li:not(.kind-select, .kind-with, .kind-explain, .kind-values, .kind-show, .kind-table) .kind {
  color: var(--pico-del-color); text-transform: uppercase; font-weight: 600; }

.result header { display: flex; flex-wrap: wrap; gap: .5rem; align-items: baseline; margin-bottom: .5rem; }
.result .which { font-family: var(--pico-font-family-monospace); }
.result .which:empty { display: none; }
.result .hint { color: var(--pico-muted-color); margin: 0; }
/* The client's table, one line per row. Lines never wrap, so its columns stay
   aligned, and a wide table scrolls sideways inside the pane. The box's
   vertical lines join at a tight line height. */
.result .scroll { overflow: auto; }
.out { padding: 0; margin: 0; list-style: none; }
.out li { width: max-content; min-width: 100%; margin: 0; padding: 0 .5rem; white-space: pre;
  font: .8rem/1.1 var(--pico-font-family-monospace); }
.out .line:empty::before { content: "\200b"; } /* keep a blank line's height */
/* One markup for every row: only an error line shows its stream. */
.out .tag { display: none; font-style: normal; font-size: .7rem; margin-right: .5rem; padding: 0 .25rem;
  border: 1px solid currentColor; border-radius: .2rem; }
/* An error wraps, so it reads whole on a phone. */
.out li:is(.s-stderr, .s-system) { width: auto; white-space: pre-wrap; overflow-wrap: anywhere;
  color: var(--pico-del-color); line-height: 1.5; }
.out li:is(.s-stderr, .s-system) .tag { display: inline; }

/* The result pane stays in view: beside the list on wide screens, at the
   bottom of the screen below it on phones, so a Run tap shows its status and
   result without scrolling. */
.content-wrapper > .tinkerdown-interactive-block:has(.result) { position: sticky; }
.content-wrapper > .tinkerdown-interactive-block:has(.result):hover { transform: none; }

@media (max-width: 999px) {
  .content-wrapper > .tinkerdown-interactive-block:has(.result) { bottom: 0; z-index: 1;
    background: var(--pico-background-color); box-shadow: 0 -1px 0 var(--pico-muted-border-color); }
  .result .scroll { max-height: 40vh; }
}

/* Wide screens: the queries on the left, the result beside them. */
@media (min-width: 1000px) {
  .content-wrapper:has(.result) { max-width: none; display: grid; grid-template-columns: minmax(0, 1fr) minmax(24rem, 60%); column-gap: 1.5rem; align-items: start; }
  .content-wrapper:has(.result) > :not(.tinkerdown-interactive-block) { grid-column: 1 / -1; }
  .content-wrapper:has(.result) > .tinkerdown-interactive-block { margin: 1rem 0; max-width: none; min-width: 0; }
  .content-wrapper > .tinkerdown-interactive-block:has(.result) { top: 1rem; }
  .result .scroll { max-height: calc(100vh - 8rem); }
}
</style>
