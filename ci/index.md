---
title: CI
# Pass the gh commands when you serve this page; see README.md for the full command:
#   tinkerdown serve --allow-exec . --source 'myapp=…' --source 'jobs=…' --source 'log=…' --action 'rerun=…' --action 'cancel=…'
sources:
  ci:
    type: merge
    inputs: ["*"]
    buffer: 200
    key: [source, fields.id]
    sort: -fields.createdAt
    drop: fields.cleared=true
    accepts:
      kinds: [cmd]
      min: 1
      hint: "One command per repo that polls gh run list; see the README"
  jobview:
    type: merge
    inputs: [jobs]
    key: [fields.name]
    drop: fields.done=true
    # ref (run ID and attempt) checks the pair from one row: each value is
    # checked against any row, so run and attempt alone would let a link
    # to an older attempt stay open.
    params: &run
      ref: {in: ci.fields.ref}
      run: {in: ci.fields.id}
      repo: {in: ci.fields.repo}
      attempt: {in: ci.fields.attempt}
  logview:
    type: merge
    inputs: [log]
    buffer: 1000
    params: *run   # the same run as jobview
---

## [All] | [Running] fields.status != completed | [Failed] fields.conclusion = failure | [Passed] fields.conclusion = success

```lvt
<main lvt-source="ci">
  <form name="Filter" role="search">
    <input type="search" name="f.fields.displayTitle" data-testid="search" placeholder="title"
           aria-label="Search runs by title" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <small data-testid="status">{{len .Data}} runs</small>
  {{if .Error}}<p data-testid="error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Inputs}}
    <p data-testid="empty">No repos yet. {{.Accepts.Hint}}.</p>
  {{end}}

  <ol class="runs" data-testid="runs">
    {{range .Data}}
      <li data-key="{{.key}}" class="st-{{.fields.status}} c-{{.fields.conclusion}} stream-{{.stream}}">
        <small class="state"><span class="status">{{.fields.status}}</span><span class="conclusion">{{.fields.conclusion}}</span></small>
        <a class="run" href="?ref={{.fields.ref}}&amp;run={{.fields.id}}&amp;repo={{.fields.repo}}&amp;attempt={{.fields.attempt}}" aria-label="Show run {{.fields.number}}">#{{.fields.number}}</a>
        <span class="what"><b class="title">{{.fields.displayTitle}}</b> <span class="meta">{{.fields.workflowName}} · {{.fields.headBranch}} · {{.source}}</span></span>
        <span class="acts">
          <button name="rerun" data-run="{{.fields.id}}" data-repo="{{.fields.repo}}" class="a-rerun outline" aria-label="Rerun the failed jobs of run {{.fields.number}}">Rerun failed</button>
          <button name="cancel" data-run="{{.fields.id}}" data-repo="{{.fields.repo}}" data-confirm="Cancel run #{{.fields.number}}?" class="a-cancel outline contrast" aria-label="Cancel run {{.fields.number}}">Cancel</button>
          <a class="gh" href="{{.fields.url}}" target="_blank" rel="noopener" aria-label="Open run {{.fields.number}} on GitHub">↗</a>
        </span>
        <span class="msg">{{.source}}: {{.msg}}</span>
      </li>
    {{end}}
  </ol>
</main>
```

```lvt
<section class="jobpane" lvt-source="jobview" aria-label="Jobs">
  <header><b data-testid="pane-title">{{with .Params.run}}Run {{.}}{{else}}Jobs{{end}}</b>{{with .Params.attempt}} <small>attempt {{.}}</small>{{end}}</header>
  {{if .Error}}<p data-testid="jobs-error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Params}}<p class="hint" data-testid="pane-hint">Click a run's number to see its jobs and the failing log.</p>{{end}}
  <ol class="jobs" data-testid="jobs">
    {{range .Data}}<li data-key="{{.key}}" class="st-{{.fields.status}} c-{{.fields.conclusion}}"><b class="job">{{.fields.name}}</b> <small class="failed">{{.fields.failed}}</small></li>{{end}}
  </ol>
</section>
```

```lvt
<section class="logpane" lvt-source="logview" aria-label="Failed log">
  {{if .Data}}<header><b>Failed log</b> <small data-testid="log-status">{{len .Data}} lines</small></header>{{end}}
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <ol class="log" data-testid="log">
    {{range .Data}}<li data-key="{{.seq}}" class="lvl-{{.level}} stream-{{.stream}}">{{.line}}</li>{{end}}
  </ol>
</section>
```

<style>
.runs { display: flex; flex-direction: column; padding: 0; list-style: none; font: .85rem/1.6 var(--pico-font-family-monospace); }
.runs li { display: grid; grid-template-columns: 6rem 4.5rem minmax(0, 1fr) auto; gap: .5rem; align-items: center; margin: 0; padding: .2rem 0; }
.runs .what { min-width: 0; overflow-wrap: anywhere; }
.runs .run { justify-self: start; }
.runs .meta { opacity: .65; font-size: .8em; }

/* A run shows its status while it runs, then its conclusion. */
.runs .st-completed .status, .runs li:not(.st-completed) .conclusion { display: none; }
.runs .state::before { display: inline-block; width: 1.2em; content: "•"; }
.runs li:not(.st-completed) { color: var(--pico-primary); }
.runs li:not(.st-completed) .state::before { content: "●"; }
.runs .c-success { color: var(--pico-ins-color); }
.runs .c-success .state::before { content: "✓"; }
.runs :is(.c-failure, .c-startup_failure, .c-timed_out) { color: var(--pico-del-color); }
.runs :is(.c-failure, .c-startup_failure, .c-timed_out) .state::before { content: "✗"; }
.runs :is(.c-cancelled, .c-skipped, .c-neutral, .c-stale) { color: var(--pico-muted-color); }

/* Only the buttons that fit the run, chosen by the row's classes: a per-row
   {{if}} would make rows differ, and the list would re-render whole. */
.runs .acts { display: flex; gap: .25rem; align-items: center; justify-content: flex-end; }
.runs .acts button { display: none; padding: .1rem .45rem; font-size: .75rem; margin: 0; }
.runs li:not(.st-completed) .a-cancel,
.runs :is(.c-failure, .c-startup_failure, .c-cancelled, .c-timed_out) .a-rerun { display: inline-block; }
.runs .gh { text-decoration: none; }

/* A failed poll: gh's message, shown first, until the next good poll. */
.runs .msg { display: none; }
.runs .stream-stderr { order: -1; display: block; color: var(--pico-del-color); }
.runs .stream-stderr > :not(.msg) { display: none; }
.runs .stream-stderr .msg { display: block; overflow-wrap: anywhere; }

.jobpane header, .logpane header { display: flex; gap: .5rem; align-items: baseline; margin-bottom: .5rem; }
.jobpane .hint { color: var(--pico-muted-color); margin: 0; }
.jobs { padding: 0; margin: 0; list-style: none; font: .85rem/1.7 var(--pico-font-family-monospace); }
.jobs li { margin: 0; }
.jobs li::before { display: inline-block; width: 1.2em; content: "●"; color: var(--pico-primary); }
.jobs .st-completed::before { content: "•"; color: var(--pico-muted-color); }
.jobs .c-success::before { content: "✓"; color: var(--pico-ins-color); }
.jobs .c-failure::before { content: "✗"; color: var(--pico-del-color); }
.jobs .failed { color: var(--pico-del-color); }
.log { max-height: 60vh; overflow-y: auto; padding: 0; margin: 0; list-style: none; font: .75rem/1.5 var(--pico-font-family-monospace); white-space: pre-wrap; }
.log li { margin: 0; overflow-wrap: anywhere; }
.log .lvl-error { color: var(--pico-del-color); }

/* Wide screens: the runs on the left, the jobs and the log beside them.
   The three blocks sit in the panel under the tabs heading. */
@media (min-width: 1000px) {
  .content-wrapper:has(.jobpane) { max-width: none; }
  .tinkerdown-tabs-content:has(.jobpane) { display: grid; grid-template-columns: minmax(0, 1fr) minmax(22rem, 42%); column-gap: 1.5rem; align-items: start; }
  /* The blocks' wide-screen bleed (negative margins) would overlap the columns. */
  .tinkerdown-tabs-content:has(.jobpane) > .tinkerdown-interactive-block { margin: 1rem 0; max-width: none; grid-column: 2; }
  .tinkerdown-tabs-content:has(.jobpane) > .tinkerdown-interactive-block:has(.runs) { grid-column: 1; grid-row: 1 / span 2; }
  .tinkerdown-tabs-content:has(.jobpane) > .tinkerdown-interactive-block:has(:is(.jobpane, .logpane)):hover { transform: none; }
  /* The runs scroll on their own, so a run far down the list opens beside
     the jobs and the log instead of scrolling them away. */
  .tinkerdown-tabs-content:has(.jobpane) .runs { max-height: calc(100vh - 16rem); overflow-y: auto; }
  .tinkerdown-tabs-content:has(.jobpane) .log { max-height: calc(100vh - 26rem); }
}

/* Phones: the title on its own line under the state, the buttons after it;
   the jobs and the log below the runs. */
@media (max-width: 700px) {
  .runs li { grid-template-columns: 6rem minmax(0, 1fr); padding: .35rem 0; border-bottom: 1px solid var(--pico-muted-border-color); }
  .runs :is(.what, .acts) { grid-column: 1 / -1; }
  .runs .acts { justify-content: flex-start; }
}
</style>
