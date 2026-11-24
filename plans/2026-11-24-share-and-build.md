# Plan: say what onefile is for, make new apps easy to build, and share them by short name

State — Done: nothing. Next: plan sign-off, then Phase 1.

## Context

onefile's pitch: put a web page over a command you already use, in minutes, and share it with a teammate as one line that's safe to run. Safety is a main selling point and gets played up (user, 2026-11-24). The repo has five apps, but the README opens with "small apps for developers" and doesn't say why to use them, what they can't do, or how to make your own.

Already built, so this plan only documents it:
- **Safety.** A page from a URL may declare only `merge` sources. It can't contain actions, schedules, includes, wasm or unsafe HTML (`checkRemotePage`, tinkerdown `cmd/tinkerdown/commands/remote_page.go`). Every command comes from the `--source` and `--action` flags of the person running it.
- **Pinning.** `serve` prints `Pin it with <url>#sha256=<hex>` after a fetch (tinkerdown `serve.go:208`).
- **Team config.** A page from a URL reads a `--config` file of the user's own ([plans/README.md](README.md#rules-every-app-follows), checked 2026-11-23). The http README already shows one (`http/README.md:24`).
- **Local only.** `serve` listens on localhost unless `--host` says otherwise (`serve.go:376`).

What's missing:
- **Short names.** `parsePageArg` treats anything that isn't `http(s)://` or `file://` as a local path, so `github.com/livetemplate/onefile/http` fails with "directory does not exist".
- **A guide for building an app.** `AGENTS.md` covers only running the log viewer. How to build a page is spread across `plans/README.md` (rules and pitfalls) and the five apps' plans, which are written for this repo's review process, not for someone building their own app.

## Product

| User journey | What they see | Built on |
|---|---|---|
| Learn what onefile is and whether it fits | README: the pitch, why, what it isn't, then the apps | Phase 1 |
| Run an app | `tinkerdown serve github.com/livetemplate/onefile/http …` | Phase 3, Phase 4 |
| Check that a teammate's line is safe before running it | README "Safe to share": what a page from a URL can't do. Every command it runs is written in the line itself. | Phase 1 |
| Share an app, or a page of your own, with a teammate | one line with its `--source` flags, pinned by `@<commit>` or `#sha256=` | Phase 1 (pin), Phase 3 (short name) |
| Build a page over your own command with an LLM | "Read AGENTS.md and build me a page over `<command>`". The page serves on the first or second try. | Phase 2, tested on db in Phase 5 |
| Remix an app, by hand or with an LLM | copy `index.md` into your repo and edit it. Pass your commands as flags, and publish the page at `github.com/you/repo/app`. With an LLM: "Read AGENTS.md and change the http app to …" | Phase 1 (README section), Phase 2 (guide) |

The top of the README after Phase 1 (on a phone it wraps; nothing in it is wider than a sentence):

```
onefile
<the pitch, one sentence: Phase 1's user contribution>

  tinkerdown serve --allow-exec \
    github.com/livetemplate/onefile/logs \
    --source 'api=docker logs -f api'

Safe to share: the page only shows. Every command it
runs is in the line you typed, and it listens on
localhost only. ...
Why: one way to build a screen over any CLI. ...
What it isn't: no accounts, no hosted server, ...

Apps | Build your own | Remix one | Share with your team
```

## Decisions

- One plan covers the onefile docs and the tinkerdown short names (user, 2026-11-24).
  alternative: a separate tinkerdown plan for short names
- The short-name form is Go-style: `github.com/<owner>/<repo>/<path>[@<ref>]` (user, 2026-11-24).
  alternative: `gh:<owner>/<repo>/<path>`, or a name for this repo only (`onefile/http`)
  because: it reads like an import path, needs no registry, and anyone can publish one
- `ref` defaults to `HEAD`.
  alternative: default to `main`
  because: `HEAD` follows each repo's own default branch (checked 2026-11-24)
- The short-name grammar is in § Contract: `@` splits at the last `@`, as in Go, and a ref can't contain `/`.
  alternative: allow `feature/x` refs
  because: a raw URL can't tell where a ref with `/` ends and the path starts, and a shared line should use a commit anyway
- Only `github.com` for now.
  alternative: also gitlab.com and codeberg.org
  because: every onefile app and every user so far is on GitHub. A second host is one more expansion rule when someone asks (G1).
- When the argument names an existing local path, the local path wins.
  alternative: a short name always wins
  because: GOPATH-style checkouts (`~/go/src/github.com/...`) are real local directories, and serving one must keep working
- A short name changes only how the URL is written. After expansion it is an ordinary page URL: the same fetch, redirect checks, size limit, `#sha256=` pin and `checkRemotePage`.
  alternative: none considered
- Errors and the pin line show both the short name and the URL it expanded to, and the pin line keeps the short form (`github.com/…/http@<ref>#sha256=…`).
  alternative: show only the expanded URL
  because: the user should be able to share back the line they typed
- `--config` is always a local file, never fetched. A teammate gets the config from the team's repo, or writes their own.
  alternative: `--config` from a URL or a short name
  because: a fetched config would be commands from a URL, which is what `checkRemotePage` exists to refuse
- Flags are the preferred way to run an app. The README pitch, each app's README and the guide lead with `--source` and `--action` flags. A `--config` file is the fallback for commands too long for one line (user, 2026-11-24).
  alternative: lead with a `--config` file
  because: a line with its flags shows every command it runs, which is the safety claim, and it needs no second file
- The guide teaches only the shareable form: the page declares a `merge` source, and the commands come from flags.
  alternative: also teach pages that declare their own exec sources, which work locally
  because: such a page fails `checkRemotePage`, so it can never be shared by URL or short name
- `AGENTS.md` stays one file with three parts, "Run an app", "Build an app" and "Remix an app", so one link serves an LLM for any of them (user, 2026-11-24).
  alternative: a separate `BUILD.md`
  because: the user asked for one file to point an LLM at
- We use the guide ourselves. It is written for someone building a page in their own repo, and onefile's own apps are built from it too. plans/README keeps only the review process, and its build step says to build the page by following AGENTS.md (user, 2026-11-24).
  alternative: a separate build guide for onefile's own apps
  because: a guide we don't use goes stale, and our next app shows where it falls short
- The page-building pitfalls move from `plans/README.md` into the build guide, and plans/README links to them.
  alternative: keep them in plans/README and link from the guide
  because: they're about building a page, which is the guide's job. Each fact stays in one place.
  Their plan links become `plans/<file>.md`, and the one inbound link (`plans/2026-11-22-http.md:220`) moves to the new anchor.
- The guide teaches by pattern, each pattern linked to the app that uses it: a live list (`containers`), streaming lines (`logs`), polling with an error row (`ci`), a click that runs a command (`http`), rerun on save (`tests`).
  alternative: none considered
- Safety is the README's second section, right after the command. It lists each guarantee: a page from a URL can't run commands, read files or hold actions; every command is in the user's own flags or config; `serve` listens on localhost; `#sha256=` pins the exact page. Each guarantee links to the tinkerdown code or test that enforces it (user, 2026-11-24).
  alternative: a safety paragraph further down, after the apps
  because: safe sharing is what sets a page apart from a shared script, and a claim with a link to its check can be verified
- The README says what onefile isn't:
  - no accounts and no hosted server, because pages run on your machine at localhost
  - no stored data beyond what the tool keeps
  - nothing to install besides tinkerdown and the tool itself
  - one file per app
  - not for production monitoring

  alternative: none considered
- The guide is tested on `db`, the next roadmap app (user, 2026-11-24). After db's plan is approved through the usual process, a fresh subagent builds the page from `AGENTS.md` and that plan alone. Where it gets stuck becomes edits to the guide, made in the db PR.
  The subagent works in db's worktree and is told not to read other plans or apps' source. It still inherits the global agent instructions, which Results records as a limit of the test.
  alternative: two throwaway pages over `du` and `ps`, built before db
  because: a real app is a stronger test than a page we throw away
- Short names ship with tinkerdown's next batched release, not a release of their own (user, 2026-11-24). Phase 4 switches the onefile docs over after that release.
  alternative: release right after Phase 3
  because: the user asked for fewer, larger releases

## Security

- What runs: nothing new. A short name expands to a URL, and the page behind it goes through `checkRemotePage` as today.
- Values from the browser: none new.
- Hand-made messages and replayed URLs: none new.
- Secrets: none new. Flags and a `--config` file hold commands, not tokens. The README says to keep tokens in the environment (`gh` and `docker` already read their own).
- Untrusted text: a short name is parsed, never passed to a shell, and § Contract's character rules refuse anything that could change the fetched path.
- What the README must warn about: a branch name like `@main` can change under you, so a shared line should use `@<commit>` or `#sha256=`. The same `github.com/` look covers a typo-squatted owner, so check the owner before running a teammate's line.

## Files

onefile (worktree `.worktrees/share-and-build`):
- `README.md`: the pitch, why, what it isn't, how sharing is safe, "Share with your team" (the line with its flags, plus a pin) and "Remix an app". Short names arrive in Phase 4.
- `AGENTS.md`, in three parts:
  - "Run an app": today's steps, for any app, not only logs.
  - "Build an app": the recipe, the patterns, the pitfalls, and publishing your page at its short name.
  - "Remix an app": copy an app by its short name, keep its patterns, change its commands and markup, and publish it under your own name.
- `plans/README.md`: the pitfalls section is replaced by a link to AGENTS.md, and step 5 of "How an app gets built" says to build the page by following AGENTS.md.
- each `<app>/README.md`, `ROADMAP.md`: short names in Phase 4.
- `e2e/*_test.go`: Phase 4. Each test's URL swap matches the short name instead of the raw URL.
- `plans/2026-11-22-http.md`: the pitfalls link at line 220 points at AGENTS.md.

tinkerdown (Phase 3, worktree `.worktrees/short-names`):
- `cmd/tinkerdown/commands/remote_page.go`: `parsePageArg` expands a short name. `pageArg` keeps the short form for messages and the pin line.
- `cmd/tinkerdown/commands/serve.go`: the pin line and help text. The help shows a short-name example.
- `cmd/tinkerdown/commands/remote_page_test.go`: the expansion cases in § Acceptance criteria.
- `docs/reference/cli.md`: short names, next to the existing `#sha256=` text.
- `CHANGELOG.md`: an `[Unreleased] ### Added` entry.

## Tracker

| # | Type | Item | Severity | Status |
|---|---|---|---|---|
| A1 | assumption | `raw.githubusercontent.com` serves a tag in place of `<ref>` as it does a branch and a short commit. | low | open (Phase 3) |
| R1 | risk | The fresh agent still can't build db after two rounds of guide edits. Then the patterns, not the guide, are too hard, and we'd record which pattern and file a tinkerdown issue for it rather than grow the guide. | medium | open (Phase 5) |
| R2 | risk | Raw GitHub caches a branch for about five minutes, so a teammate can get an older page right after a push. Mitigation: shared lines use `@<commit>`. | low | accepted |
| G1 | gap | Hosts other than github.com. | low | unowned |

## Acceptance criteria

- [ ] `/simplify` has run on each phase's diff.
- [ ] tinkerdown `go test ./cmd/tinkerdown/commands/...` covers:
  - `github.com/o/r/app` → `…/o/r/HEAD/app/index.md`
  - `@v1`, `@<sha>` and `@main` as refs
  - `github.com/o/r/doc.md`
  - `github.com/o/r` (root `index.md`)
  - `#sha256=` after a short name
  - an existing local `github.com/...` directory still served locally
  - refusals for `github.com/o` (no repo), `..`, `?`, `%` and an empty ref
  - an error message that names both the short name and the URL
- [ ] tinkerdown's full suite (`go test -timeout 40m ./...`) and pre-commit pass.
- [ ] `cd e2e && go test ./...` in onefile passes with each app's README command in its short-name form, after Phase 4. The tests swap the short name for a local `httptest` URL as they swap the raw URL today, so they stay offline. The expansion itself is covered by tinkerdown's unit tests, and the real fetch by the manual run.
- [ ] One manual run of each changed README command from GitHub by its short name, recorded in Results.
- [ ] A fresh subagent, given only `AGENTS.md` and db's approved plan, builds a db page that passes `cd e2e && go test -run TestDB ./...`. Where it got stuck, and how many tries it took, are in Results.
- [ ] `grep -rn 'raw.githubusercontent' README.md AGENTS.md */README.md` finds only the "what a short name expands to" line after Phase 4.
- [ ] Each safety guarantee in the README links to the tinkerdown code or test that enforces it (`checkRemotePage` and its tests, the `--host` default, the `#sha256=` check), and each link opens.
- [ ] `prereview check` passes on every changed doc.

## Contract: short names

```
ARG   = "github.com/" OWNER "/" REPO [ "/" PATH ] [ "@" REF ] [ "#sha256=" HEX64 ]
OWNER = [A-Za-z0-9-]{1,39}, not starting or ending with "-"
REPO  = [A-Za-z0-9._-]+, not "." or ".."
PATH  = segments of [A-Za-z0-9._-]+ joined by "/", no "." or ".." segment
REF   = [A-Za-z0-9._-]+, not starting with "." or "-", no ".."   (default HEAD)
→ https://raw.githubusercontent.com/OWNER/REPO/REF/PATH/index.md
  (PATH ending in .md: used as is; no PATH: REF/index.md)
```

`#` is split off first, then the last `@`. Anything else, such as `?`, `%`, a space or an empty part, is refused with a message that names the rule.

## Phases

### Phase 1: pitch, scope and sharing (onefile)

- Audit: read the README as someone who has never heard of tinkerdown, and list what they can't answer.
- Implementation: `README.md` per the Files list. **User contribution:** the one-sentence pitch at the top of the README. It's the line people decide on, and it should sound like you.
- Acceptance: `prereview check`, and the README's commands run as written.
- Learn:

### Phase 2: the build guide (onefile)

- Audit: read the five apps' pages and plans, and list each pattern and pitfall that a new page needs.
- Implementation: `AGENTS.md` and `plans/README.md` per the Files list.
- Acceptance: `prereview check`, and plans/README has no page-building advice left that isn't in AGENTS.md.
- Learn:

### Phase 3: short names (tinkerdown)

- Audit: A1. Trace `parsePageArg` → `fetchRemotePage` → the pin line, and confirm a short name reaches every place that prints `shown`.
- Implementation: per the tinkerdown Files list. Merge to main; it ships with tinkerdown's next release.
- Acceptance: the tinkerdown criteria.
- Learn:

### Phase 4: switch the docs to short names (onefile)

- Starts after the tinkerdown release that carries Phase 3.
- Implementation: every README command and `AGENTS.md` use short names, and the README names the new minimum tinkerdown. Each e2e test swaps the short name for its local URL.
- Acceptance: the onefile e2e and grep criteria.
- Learn:

### Phase 5: build db from the guide (onefile)

- Starts after db's plan is approved. It can run before or after Phases 3 and 4.
- Implementation: a fresh subagent builds db from `AGENTS.md` and db's plan. Guide fixes go in the db PR.
- Acceptance: the db criterion.
- Learn:

## Results

<!-- Filled in after Phase 5: the db build by a fresh agent, and what each assumption turned out to be. -->
