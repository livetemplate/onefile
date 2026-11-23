---
title: HTTP
# Pass your .http file and the curl command when you serve this page; see README.md for the full command:
#   tinkerdown serve --allow-exec . --source 'api=http:./api.http' --source 'send=…curl…'
sources:
  requests:
    type: merge
    inputs: ["*"]
    key: [fields.entry]
    accepts:
      kinds: [http]
      min: 1
      hint: "--source api=http:./your.http; see the README"
  response:
    type: merge
    inputs: [send]
    on: send
    buffer: 5000
    line_max: 1MiB
    params:
      entry: {in: requests.fields.entry}
---

# HTTP

```lvt
<main class="reqpane" lvt-source="requests">
  <form name="Filter" role="search">
    <input type="search" name="f.fields.text" data-testid="search" placeholder="name, method or URL"
           aria-label="Search requests" lvt-on:input="Filter" lvt-mod:debounce="250">
  </form>
  <small data-testid="status">{{len .Data}} requests</small>
  {{if .Error}}<p data-testid="error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Inputs}}
    <p data-testid="empty">No requests yet. {{.Accepts.Hint}}.</p>
  {{end}}

  <ol class="reqs" data-testid="requests">
    {{range .Data}}
      <li data-key="{{.key}}" data-testid="req-{{.fields.entry}}">
        <b class="name">{{.fields.name}}</b>
        <button name="send" data-entry="{{.fields.entry}}" data-testid="send-{{.fields.entry}}" class="outline"
                aria-label="Send {{.fields.name}}">Send</button>
        <span class="target"><code class="method m-{{.fields.method}}">{{.fields.method}}</code> <span class="url">{{.fields.url}}</span></span>
        <details class="sent"><summary>request</summary><pre>{{.fields.headers}}

{{.fields.body}}</pre></details>
      </li>
    {{end}}
  </ol>
</main>
```

```lvt
<aside class="resp" lvt-source="response" aria-label="Response">
  <header>
    <b>Response</b>
    <small data-testid="state" aria-busy="{{if .Running}}true{{else}}false{{end}}">{{if .Running}}sending…{{else}}{{len .Data}} lines{{end}}</small>
  </header>
  {{if .Error}}<p data-testid="resp-error"><mark>{{.Error}}</mark></p>{{end}}
  {{if not .Data}}<p class="hint" data-testid="hint">Press Send on a request.</p>{{end}}
  <ol class="body" data-testid="response">
    {{range .Data}}
      <li data-key="{{.seq}}" data-exit="{{.fields.exitcode}}" class="s-{{.stream}}">
        <b class="sum">{{.fields.http_code}} · {{printf "%.3f" .fields.time_total}} s</b>
        <pre class="line">{{linkify (jsonindent .line)}}</pre>
      </li>
    {{end}}
  </ol>
</aside>
```

<style>
.reqs { padding: 0; list-style: none; }
.reqs li { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .1rem .75rem; align-items: center;
  padding: .4rem 0; border-bottom: 1px solid var(--pico-muted-border-color); }
.reqs .name { min-width: 0; overflow-wrap: anywhere; }
.reqs button { padding: .1rem .6rem; font-size: .8rem; margin: 0; }
.reqs .target { grid-column: 1 / -1; min-width: 0; overflow-wrap: anywhere; font: .8rem/1.5 var(--pico-font-family-monospace); }
.reqs .method { font-weight: 600; }
.reqs :is(.m-POST, .m-PUT, .m-PATCH) { color: var(--pico-ins-color); }
.reqs .m-DELETE { color: var(--pico-del-color); }
.reqs .sent { grid-column: 1 / -1; margin: 0; font-size: .8rem; }
.reqs .sent summary { color: var(--pico-muted-color); }
.reqs .sent pre { margin: .25rem 0 0; padding: .5rem; white-space: pre-wrap; overflow-wrap: anywhere; }

.resp header { display: flex; gap: .5rem; align-items: baseline; margin-bottom: .5rem; }
.resp .hint { color: var(--pico-muted-color); margin: 0; }
/* One markup for every row. curl's summary line (the one with an exitcode)
   moves to the top and shows only the status and time; every other row shows
   only its line. A per-row {{if}} would make rows differ. */
.body { display: flex; flex-direction: column; padding: 0; margin: 0; list-style: none; }
.body li { margin: 0; padding: 0; }
/* Each line is its own pre element: drop the code-block box tinkerdown
   gives one, so the lines read as one response. */
.body .line { margin: 0; padding: 0 .5rem; border: 0; border-radius: 0; box-shadow: none; background: none;
  color: var(--pico-color); white-space: pre-wrap; overflow-wrap: anywhere; font: .8rem/1.5 var(--pico-font-family-monospace); }
.body .line:empty::before { content: "\200b"; } /* keep a blank line's height */
.body .line a { display: inline; border: 0; }
.body .sum { display: none; }
.body li:not([data-exit=""]) { order: -1; margin-bottom: .5rem; }
.body li:not([data-exit=""]) .sum { display: block; }
.body li:not([data-exit=""]) .line { display: none; }
.body li:is(.s-stderr, .s-system) .line { color: var(--pico-del-color); }

/* Wide screens: the requests on the left, the response beside them and kept
   in view. */
@media (min-width: 1000px) {
  .content-wrapper:has(.resp) { max-width: none; display: grid; grid-template-columns: minmax(0, 1fr) minmax(24rem, 55%); column-gap: 1.5rem; align-items: start; }
  .content-wrapper:has(.resp) > :not(.tinkerdown-interactive-block) { grid-column: 1 / -1; }
  .content-wrapper:has(.resp) > .tinkerdown-interactive-block { margin: 1rem 0; max-width: none; }
  .content-wrapper > .tinkerdown-interactive-block:has(.resp) { position: sticky; top: 1rem; }
  .content-wrapper > .tinkerdown-interactive-block:has(.resp):hover { transform: none; }
  .resp .body { max-height: calc(100vh - 8rem); overflow-y: auto; }
}
</style>
