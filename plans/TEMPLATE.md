# Plan: a <app> onefile app, <what it does in a few words>

State — Done: nothing. Next: plan sign-off, then Phase 1.

<!-- Copy to plans/YYYY-MM-DD-<app>.md. Replace each comment with the content it asks for, then delete the comment. Every section is required unless its comment says otherwise. Keep it under 250 lines a reviewer must judge, and don't repeat what plans/README.md already says. -->

## Context

<!--
- One paragraph: what the user sees and does. Link the app's ROADMAP.md row for why it ranks where it does; don't restate the stars.
- Rules from review that shaped the design, each with its date: "(user, YYYY-MM-DD)".
- The audit: the tool's version and the date, then each fact about its real output that the design depends on (fields, formats, limits, surprises).
- What tinkerdown can't do yet that the app needs, one line each. Each becomes an upstream phase.
-->

## Product

<!-- One row per user journey. "Built on" names the source, flag or template feature, and marks the upstream ones with their phase. -->

| User journey | What they see | Built on |
|---|---|---|
| | | |

<!-- A desktop mockup and a phone mockup, each in a code block, each narrow enough to read on a phone. -->

## What the code looks like

<!--
- The serve command, exactly as the README will publish it. Users copy this command, so it's the app's contract.
- The page's frontmatter sources. Add markup only where a reviewer must judge it (a button's values, a row's classes).
- Each upstream feature in a few lines: its config or flag, and what it does.
Say that the audits can change the details.
-->

## Decisions

<!--
- <decision>
  alternative: <what you rejected>   (or: none considered)
  because: <reason>                  (omit when none)
Put the app's own choices first, then each upstream feature's.
-->

## Files

onefile (worktree `.worktrees/<app>`):
- `<app>/index.md` new?:
- `<app>/README.md` new?, `<app>/docs/desktop.png`, `phone.png` new?:
- `e2e/<app>_test.go` new?, `e2e/testdata/<app>/` new?:
- `README.md`, `ROADMAP.md`:

<!-- One block per upstream repo, with its worktree: "tinkerdown (Phase 1, worktree `.worktrees/<feature>`):". Mark each path that doesn't exist yet `new?`. Each file needs at least one decision or acceptance criterion. Leave the section out when the app needs no upstream change. -->

## Tracker

<!-- Assumptions (A) to check before building, risks (R) you'd act on, and gaps (G) that no phase owns. Give a gap's owner as an issue link once one is filed. -->

| # | Type | Item | Severity | Status |
|---|---|---|---|---|
| A1 | assumption | | | open (Phase 1) |

## Acceptance criteria

<!-- Only what's specific to this app. Each names a command, test or symbol. Never tick a box you haven't run. -->

- [ ] `<upstream repo>` `go test ./...`: <the cases>
- [ ] `cd e2e && go test -run Test<App> ./...`: <the journeys from § Product, one case each>
- [ ] And every check in [plans/README.md](README.md#checks-every-app-passes).

## Constraints

<!-- Optional: only what this app must not break beyond the rules in plans/README.md. -->

## Contract

<!-- The row shapes the page reads, each enum's members listed in full, the action and click values, and the defaults of every tunable. -->

## Phases

### Phase 1: <upstream feature> (tinkerdown)

- Audit: <the assumptions this phase checks, and the code paths to trace first>
- Implementation: per the tinkerdown Files list. **User contribution:** <5–10 lines where a design choice is the user's>
- Acceptance: the tinkerdown criteria, then a release through `release.sh`.
- Learn:

### Phase 2: the app (onefile)

- Audit: the remaining assumptions, each with a throwaway check. Capture the tool's real output for the test fixtures. Check the Contract against what Phase 1 shipped, and write any drift into this plan in the same commit.
- Implementation: per the onefile Files list.
- Acceptance: the onefile criteria.
- Learn:

## Results

<!-- Filled in after Phase 2: the manual run against the real tool, and what each assumption turned out to be. -->
