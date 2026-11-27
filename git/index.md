---
title: Git
# Serve this page in a git repo; see README.md for what each flag runs:
#   GIT_LITERAL_PATHSPECS=1 GIT_OPTIONAL_LOCKS=0 tinkerdown serve --allow-exec github.com/livetemplate/onefile/git \
#     --source 'status=git status -z --no-renames --untracked-files=all | tr "\000" "\n"' \
#     --source 'branches=git for-each-ref --format="%(HEAD)%09%(refname:short)%09%(upstream:track,nobracket)%09%(committerdate:format:%m-%d)%09%(contents:subject)" refs/heads' \
#     --source 'log=git log -n 100 --date=format:%m-%d --format="%H%x09%h%x09%cd%x09%an%x09%D%x09%s"' \
#     --source 'diff=...' --source 'commit=env -u GIT_LITERAL_PATHSPECS git commit -m "$TD_message"' \
#     --action 'stage=git add -- "$TD_path"' --action 'unstage=git reset -q -- "$TD_path"' \
#     --action 'switch=git switch -- "$TD_branch"'
sources:
  files:
    type: merge
    inputs: [status]
    every: 2s
    key: [fields.path]
    pattern: '^(?P<x>.)(?P<y>.) (?P<path>.+)$'
  refs:
    type: merge
    inputs: [branches]
    every: 2s
    key: [fields.name]
    pattern: '^(?P<head>.)\t(?P<name>[^\t]+)\t(?P<track>[^\t]*)\t(?P<when>[^\t]*)\t(?P<subject>.*)$'
  commits:
    type: merge
    inputs: [log]
    every: 2s
    key: [fields.hash]
    pattern: '^(?P<hash>[0-9a-f]+)\t(?P<short>[^\t]*)\t(?P<when>[^\t]*)\t(?P<author>[^\t]*)\t(?P<refs>[^\t]*)\t(?P<subject>.*)$'
  filediff:
    type: merge
    inputs: [diff]
    every: 2s
    buffer: 5000
    pattern: '^(?P<mark>[-+@]?)'
    params:
      path: {in: files.fields.path}
  committed:
    type: merge
    inputs: [commit]
    on: commit
    params:
      message: {typed: true}
actions:
  stage:
    values:
      path: {in: files.fields.path}
  unstage:
    values:
      path: {in: files.fields.path}
  switch:
    values:
      branch: {in: refs.fields.name}
---

# Git

```lvt
<section class="changes" lvt-source="files" aria-label="Changes">
  <header><b>Changes</b> <small>{{len .Data}}</small></header>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <ol class="files">
    {{range .Data}}
      <li data-key="{{.key}}" data-x="{{.fields.x}}" data-y="{{.fields.y}}">
        <code class="xy">{{.fields.x}}{{.fields.y}}</code>
        <a class="path" href="?path={{urlquery .fields.path}}">{{.fields.path}}</a>
        <button name="stage" data-path="{{.fields.path}}" class="stage outline">Stage</button>
        <button name="unstage" data-path="{{.fields.path}}" class="unstage outline secondary">Unstage</button>
      </li>
    {{end}}
  </ol>
  {{if and (not .Data) (not .Error)}}<p class="hint">Nothing to commit.</p>{{end}}
</section>
```

```lvt
<section class="commit" lvt-source="committed" aria-label="Commit">
  <form>
    <input name="message" maxlength="256" placeholder="Commit message" aria-label="Commit message" required>
    {{if .Running}}<button disabled aria-busy="true">Committing</button>{{else}}<button name="commit">Commit</button>{{end}}
  </form>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <ol class="out">
    {{range .Data}}
      <li data-key="{{.seq}}" class="s-{{.stream}}">{{.line}}</li>
    {{end}}
  </ol>
</section>
```

```lvt
<aside class="diff" lvt-source="filediff" aria-label="Diff">
  <header><b class="which">{{.Params.path}}</b></header>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Params}}<p class="hint">Click a file to see its diff.</p>{{end}}
  <div class="scroll">
    <ol class="out">
      {{range .Data}}
        <li data-key="{{.seq}}" data-mark="{{.fields.mark}}">{{.line}}</li>
      {{end}}
    </ol>
  </div>
</aside>
```

```lvt
<section class="branches" lvt-source="refs" aria-label="Branches">
  <header><b>Branches</b></header>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <ol class="refs">
    {{range .Data}}
      <li data-key="{{.key}}" data-head="{{.fields.head}}">
        <b class="name">{{.fields.name}}</b>
        <small class="track">{{.fields.track}}</small>
        <button name="switch" data-branch="{{.fields.name}}" class="switch outline">Switch</button>
        <span class="subject"><small>{{.fields.when}}</small> {{.fields.subject}}</span>
      </li>
    {{end}}
  </ol>
</section>
```

```lvt
<section class="log" lvt-source="commits" aria-label="Log">
  <header><b>Log</b></header>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <ol class="commits">
    {{range .Data}}
      <li data-key="{{.key}}">
        <code class="short">{{.fields.short}}</code>
        <small class="when">{{.fields.when}}</small>
        <span class="author">{{.fields.author}}</span>
        <span class="refs">{{.fields.refs}}</span>
        <span class="subject">{{.fields.subject}}</span>
      </li>
    {{end}}
  </ol>
</section>
```

<style>
section header, aside header { display: flex; gap: .5rem; align-items: baseline; margin-bottom: .25rem; }
.hint { color: var(--pico-muted-color); margin: 0; }
ol.files, ol.refs, ol.commits, ol.out { padding: 0; margin: 0; list-style: none; }
.files li, .refs li { padding: .3rem 0; border-bottom: 1px solid var(--pico-muted-border-color); }
.files .xy, .commits .short { font-size: .8rem; padding: .05rem .3rem; }

/* A changed file: its two status letters, its path, and whichever button applies. */
.files li { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; gap: .5rem; align-items: center; }
.files .xy { white-space: pre; }
.files .path { min-width: 0; overflow-wrap: anywhere; font-family: var(--pico-font-family-monospace); font-size: .85rem; }
.files button, .refs button { padding: .1rem .6rem; font-size: .8rem; margin: 0; }
/* One markup for every row: a file with nothing unstaged has no Stage, and
   one with nothing staged has no Unstage. */
.files li[data-y=" "] .stage, .files li[data-x=" "] .unstage, .files li[data-x="?"] .unstage { display: none; }

.commit form { display: flex; gap: .5rem; margin: .75rem 0 .25rem; }
.commit input { margin: 0; }
.commit button { margin: 0; width: auto; }
.commit .out li { white-space: pre-wrap; overflow-wrap: anywhere; font: .8rem/1.4 var(--pico-font-family-monospace); }
.commit .out li.s-stderr, .commit .out li.s-system { color: var(--pico-del-color); }

.diff .which { font-family: var(--pico-font-family-monospace); overflow-wrap: anywhere; }
.diff .scroll { overflow: auto; max-height: 50vh; }
.diff .out li { width: max-content; min-width: 100%; white-space: pre; padding: 0 .5rem; font: .8rem/1.35 var(--pico-font-family-monospace); }
.diff .out li[data-mark="+"] { color: var(--pico-ins-color); }
.diff .out li[data-mark="-"] { color: var(--pico-del-color); }
.diff .out li[data-mark="@"] { color: var(--pico-muted-color); }
.diff .out li:empty::before { content: "\200b"; }

.refs li { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: .1rem .75rem; align-items: center; }
.refs .name { overflow-wrap: anywhere; }
.refs li[data-head="*"] .name::before { content: "* "; color: var(--pico-primary); }
.refs li[data-head="*"] .switch { visibility: hidden; }
.refs .subject { grid-column: 1 / -1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--pico-muted-color); }

.commits li { padding: .25rem 0; border-bottom: 1px solid var(--pico-muted-border-color); overflow-wrap: anywhere; font-size: .9rem; }
.commits .when, .commits .author { color: var(--pico-muted-color); }
.commits .refs { font-size: .8rem; color: var(--pico-primary); }
.commits .refs:empty { display: none; }

/* Wide screens: changes, commit and branches on the left; the diff and the
   log on the right. On a phone the blocks stack in page order. */
@media (min-width: 1000px) {
  .content-wrapper:has(.changes) { max-width: none; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr);
    column-gap: 1.5rem; align-items: start; grid-auto-flow: row dense; }
  .content-wrapper:has(.changes) > :not(.tinkerdown-interactive-block) { grid-column: 1 / -1; }
  .content-wrapper:has(.changes) > .tinkerdown-interactive-block { margin: .5rem 0; max-width: none; min-width: 0; grid-column: 1; }
  .content-wrapper > .tinkerdown-interactive-block:has(.diff),
  .content-wrapper > .tinkerdown-interactive-block:has(.log) { grid-column: 2; }
  .content-wrapper > .tinkerdown-interactive-block:has(.diff) { grid-row: span 2; }
  .diff .scroll { max-height: 60vh; }
}
</style>
