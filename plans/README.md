# Plans

Each app gets one plan here before any code: copy [TEMPLATE.md](TEMPLATE.md) to `plans/YYYY-MM-DD-<app>.md`. The plan covers the app and any tinkerdown or livetemplate change it needs, so all of an app's work is planned in one place. The rules, pitfalls and checks below apply to every app, so a plan links here instead of repeating them.

In a plan, a path under a `tinkerdown:` or `livetemplate:` heading is in that repo. Issue and PR numbers always name their repo (`tinkerdown#354`), since a bare `#354` here means a onefile one.

## How an app gets built

1. Take the top app from [ROADMAP.md](../ROADMAP.md) that isn't built yet.
2. Audit the tools the app will run, and write the plan from the template.
3. Run `/simplifyplan`, open the plan PR in onefile, and run `/planprcommentsfix`. Merge only after sign-off.
4. If the app needs upstream changes, build them first, each from a worktree in that repo's `.worktrees/`. Open a PR, get sign-off, and release through `release.sh`. Then open the Homebrew formula PR. Each of these needs its own sign-off.
5. Build the app from `onefile/.worktrees/<app>` against the released tinkerdown. Open one PR, and merge it after sign-off.
6. Write each phase's surprises into its Learn line, and the manual run into Results. Update the State line in the same commit as the work.

## Rules every app follows

- **One file.** The app is `<app>/index.md`, and the user downloads nothing else (user, 2026-11-23).
- **Commands come from the user.** A page served from its URL declares only `merge` sources. Every command comes from the user's own `--source` and `--action` flags, as written in the app's README. When the commands are long, the README also gives them as a `--config` file of the user's own, which a page from a URL reads too (checked 2026-11-23, [http](2026-11-22-http.md)).
- **Tools already on the machine.** Commands use what's already installed, such as `sh`, `curl` and `awk`, or the service's own CLI (`docker`, `gh`, `kubectl`). A tool the user must install for this app alone doesn't qualify (user, 2026-11-23).
- **tinkerdown stays generic.** Nothing in tinkerdown knows the app's tool or protocol, and tinkerdown never does the tool's job: for `http` it starts curl and never sends a request itself (user, 2026-11-23). What an app is missing gets built upstream as a general feature.
- **Clicks, not links, run commands with side effects.** Reloads, Back and shared links replay a URL, so a URL never starts a command that changes anything.
- **Quote and check values.** Every documented command quotes its values (`"$TD_name"`). Before calling the tool, an action command checks its values with `case` patterns, since an action has no `in` check ([tinkerdown#362](https://github.com/livetemplate/tinkerdown/issues/362)).
- **Rank by need.** The roadmap's order comes from developer need, never from what tinkerdown can do today.

## Pitfalls found so far

- A per-row `{{if}}` ends row-by-row updates. Give each row the same markup, and show or hide parts with CSS on a class from the row's fields ([containers](2026-11-21-containers.md), Phase 4).
- Key rows by a field that stays the same across updates (`key`), never by `seq`, which changes on every update ([test-runner](2026-11-21-test-runner.md), Phase 0).
- `in` and `drop` compare numbers as `fmt.Sprint` prints them, so a large numeric ID never matches. Emit IDs as strings ([tinkerdown#353](https://github.com/livetemplate/tinkerdown/issues/353)).
- Because a `--source` flag can't set `restart`, a command that finishes printing ends with `exec tail -f /dev/null`. Otherwise it's restarted and prints everything again ([tinkerdown#354](https://github.com/livetemplate/tinkerdown/issues/354)). A source in a `--config` file can set `restart: never` instead.
- `params` values are checked one by one, not as one row. When several must come from the same row, join them into one field ([tinkerdown#360](https://github.com/livetemplate/tinkerdown/issues/360)).
- A failing poll should still show on the page. Join the tool's error into one stderr line, key it `[source, ""]`, and remove it with `drop` when the next poll prints `{"cleared":true}` ([ci](2026-11-22-ci.md), [tinkerdown#361](https://github.com/livetemplate/tinkerdown/issues/361)).
- An action's error is cleared by the block's next push, so e2e tests check refusals in the server log ([tinkerdown#359](https://github.com/livetemplate/tinkerdown/issues/359)).
- A tool's real output differs from its docs. `gh run view --log-failed` printed whole job logs, `UNKNOWN STEP` in every line and a byte-order mark ([ci](2026-11-22-ci.md)). Audit the real output with the tool's version and the date, and build the test fixtures from it.
- Every key of a row becomes template state, so a row can't carry a secret. Values a command needs but the page mustn't see are kept as a row's private values (`Hub.Private`), which only a click passes on ([http](2026-11-22-http.md), Phase 1).
- tinkerdown's docs have listed flags and functions that don't exist ([log-fields](2026-11-22-log-fields.md), Phase 2). Check a docs page against the code before relying on it.

## Checks every app passes

A plan's acceptance criteria list only what is specific to the app, and end with "and every check in plans/README.md".

- [ ] `/simplify` has run on each phase's diff.
- [ ] `cd e2e && go test -run Test<App> ./...` runs the README's serve command through `sh`, as published, in Docker Chrome. A fake tool first on `PATH`, or an `httptest` server, stands in for the real service, so no test uses the network. The test captures console logs, server logs, WebSocket frames and the rendered HTML, and it is never skipped.
- [ ] A value holding `<script>` or `<img src=x onerror=alert(1)>` renders as text.
- [ ] A hand-made message with a value that isn't from a row runs nothing.
- [ ] On a phone no cell leaves its row, and the browser console has no errors.
- [ ] No process the page started remains 30 seconds after the page leaves.
- [ ] The full suite passes: `cd e2e && go test ./...`.
- [ ] One manual run of the README command against the real tool, recorded in the plan's Results.
- [ ] Served from its raw GitHub URL, the page loads with the README command copied as published.
- [ ] `<app>/README.md` has the serve command, a table of its flags, and what the page can show that might be secret. `<app>/docs/desktop.png` and `phone.png` come from chromedp.
- [ ] The app has a row in `README.md` and in ROADMAP.md's Built table. If it needs a newer tinkerdown, AGENTS.md names the new minimum.
- [ ] Each upstream change passes its repo's full suite and pre-commit hook, and has docs and a CHANGELOG entry.
- [ ] `prereview check` passes on every changed doc.
