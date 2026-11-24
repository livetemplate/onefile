# Plans

Each app gets one plan here before any code: copy [TEMPLATE.md](TEMPLATE.md) to `plans/YYYY-MM-DD-<app>.md`. The plan covers the app and any tinkerdown or livetemplate change it needs, so all of an app's work is planned in one place. The rules and checks below apply to every app, so a plan links here instead of repeating them. How to build the page itself, with its rules and pitfalls, is in [AGENTS.md](../AGENTS.md#build-an-app).

In a plan, a path under a `tinkerdown:` or `livetemplate:` heading is in that repo. Issue and PR numbers always name their repo (`tinkerdown#354`), since a bare `#354` here means a onefile one.

## How an app gets built

1. Take the top app from [ROADMAP.md](../ROADMAP.md) that isn't built yet.
2. Audit the tools the app will run, and write the plan from the template.
3. Run `/simplifyplan`, open the plan PR in onefile, and run `/planprcommentsfix`. Merge only after sign-off.
4. If the app needs upstream changes, build them first, each from a worktree in that repo's `.worktrees/`. Open a PR, get sign-off, and release through `release.sh`. Then open the Homebrew formula PR. Each of these needs its own sign-off.
5. Build the app from `onefile/.worktrees/<app>` against the released tinkerdown, following [AGENTS.md](../AGENTS.md#build-an-app). Open one PR, and merge it after sign-off.
6. Write each phase's surprises into its Learn line, and the manual run into Results. Update the State line in the same commit as the work.

## Rules every app follows

- **The page follows [AGENTS.md](../AGENTS.md#rules).** One file, commands from the user, tools already on the machine, clicks for side effects, and quoted, checked values.
- **tinkerdown stays generic.** Nothing in tinkerdown knows the app's tool or protocol, and tinkerdown never does the tool's job: for `http` it starts curl and never sends a request itself (user, 2026-11-23). What an app is missing gets built upstream as a general feature.
- **Rank by need.** The roadmap's order comes from developer need, never from what tinkerdown can do today.

## Pitfalls found so far

The page-building pitfalls are in [AGENTS.md](../AGENTS.md#pitfalls). One is about testing:

- An action's error is cleared by the block's next push, so e2e tests check refusals in the server log ([tinkerdown#359](https://github.com/livetemplate/tinkerdown/issues/359)).

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
