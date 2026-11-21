---
title: Tests
# Pass your test command when you serve this page; it must write go test -json:
#   tinkerdown serve --allow-exec . --source 'go=watchexec -e go -- go test -json ./...'
sources:
  tests:
    type: merge
    inputs: ["*"]
    buffer: 5000
    key: [source, fields.Package, fields.Test]
    collect: fields.Output
    collect_reset: fields.Action=run|start
    accepts:
      kinds: [cmd]
      min: 1
      hint: "A command that writes go test -json, such as: watchexec -e go -- go test -json ./..."
---

## [All] | [Failed] fields.Action = fail | [Passed] fields.Action = pass | [Skipped] fields.Action = skip

```lvt
<main lvt-source="tests">
  <form name="Filter" role="search">
    <input type="search" name="f.fields.Test" data-testid="search" placeholder="test name"
           aria-label="Search tests" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <small data-testid="status">{{len .Data}} rows</small>
  {{if not .Inputs}}
    <p data-testid="empty">No test command yet. {{.Accepts.Hint}}.</p>
  {{end}}

  <ol class="tests" data-testid="tests">
    {{range .Data}}
      <li data-key="{{.key}}" class="act-{{.fields.Action}}">
        <small class="act">{{.fields.Action}}</small>
        <span class="pkg">{{.fields.Package}}</span>
        <b class="name">{{.fields.Test}}</b>
        <small class="time">{{.fields.Elapsed}}</small>
        <pre class="out">{{.collected}}</pre>
      </li>
    {{end}}
  </ol>
</main>
```

<style>
.tests { padding: 0; list-style: none; font: .85rem/1.5 var(--pico-font-family-monospace); }
.tests li { display: grid; grid-template-columns: 3.5rem 10rem 1fr auto; gap: .5rem; }
.tests .act::before { display: inline-block; width: 1.2em; }
.tests .act-pass .act::before { content: "✓"; }
.tests .act-fail .act::before { content: "✗"; }
.tests .act-skip .act::before { content: "–"; }
.tests .act-run .act::before, .tests .act-output .act::before { content: "…"; }
.tests .act-pass { color: var(--pico-ins-color); }
.tests .act-skip, .tests .act-run, .tests .act-output { color: var(--pico-muted-color); }
.tests .act-fail { color: var(--pico-del-color); }
.tests .time:not(:empty)::after { content: "s"; }
.tests .out { grid-column: 1 / -1; margin: 0; white-space: pre-wrap; }
.tests li:not(.act-fail) .out, .tests .out:empty { display: none; }
@media (max-width: 600px) {
  .tests li { grid-template-columns: auto 1fr auto; }
  .tests .pkg { grid-column: 1 / -1; grid-row: 1; }
}
</style>
