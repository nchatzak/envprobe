---
description: Run the full plan → implement → review → simplify → present pipeline
argument-hint: <feature description, or a path to a brief file>
---

## Input

`$ARGUMENTS` may be any of:

- **Inline text**, short or long — a full paragraph is fine.
- **A path to a file** (`specs/rate-limit.md`, `docs/briefs/auth.md`). If it
  resolves to an existing file, read it; that file is the brief.
- **An `@` reference** (`@specs/rate-limit.md`) — already inlined for you.
- **Empty.** Look for `.pipeline/request.md`. If that's missing too, ask the
  human for the feature description and stop.

A file is the better path for anything with structure — acceptance criteria,
example payloads, links to tickets, API shapes. Don't try to compress a real
spec into one line.

Write the resolved brief to `.pipeline/request.md` verbatim before doing
anything else. Every later stage reads it from there, so a file path and
inline text behave identically from that point on. Do not summarise or
rewrite it on the way in — if the human wrote acceptance criteria, the
planner needs them exactly as written.

## Setup

**Check the working tree is clean** before anything else:

```bash
git status --porcelain
```

If it shows uncommitted changes, **stop and tell the human.** Do not proceed
without an explicit go-ahead. Every gate in this pipeline diffs against
`HEAD`, so pre-existing edits get attributed to the implementer:

- their work-in-progress counts as "the diff" for placeholder, secret, and
  erosion scans — false failures the implementer cannot fix
- the CRAP baseline is taken from a tree that already contains uncommitted
  changes, so those changes are silently laundered into "pre-existing"
- at the end, `git diff` mixes their work with the agent's, and the review
  step they're supposed to do becomes much harder

Suggest committing or stashing first. If they insist on continuing anyway,
say plainly that the diff will be mixed and the gates will be noisy.

**Then check for an interrupted run.** Read `.pipeline/state.json` if it
exists.

If it exists and `stage` is not `done`, a previous feature stopped partway.
**Stop immediately. Do not archive anything.** Report to the human:

- the feature it was working on (`feature` field)
- the stage it reached and when (`stage`, `updated`)
- attempts used so far
- whether `git status` shows uncommitted work from that run

Then ask which they want:

1. **Resume** — pick up from the recorded stage. The working tree still holds
   the code; nothing was ever committed.
2. **Abandon** — archive it and start the new request.

Wait for an answer. Never choose for them. An interrupted run holds the
approved plan, the pre-implementation CRAP baseline, and the review history —
the baseline in particular cannot be regenerated once the code has changed,
because taking it now would launder that run's regressions into
"pre-existing."

Only when the human says abandon (or when no `state.json` exists):

```bash
mkdir -p .pipeline
if [ -n "$(ls -A .pipeline 2>/dev/null | grep -v '^prev$')" ]; then
  rm -rf .pipeline/prev && mkdir -p .pipeline/prev
  find .pipeline -maxdepth 1 -type f -exec mv {} .pipeline/prev/ \;
fi
```

### Keeping state

Write `.pipeline/state.json` after **every** stage transition, before
delegating:

```json
{
  "feature": "one-line summary of the request",
  "stage": "planning | awaiting_approval | implementing | reviewing | simplifying | done",
  "plan_revision": 0,
  "attempts": 0,
  "updated": "2026-08-22T14:30:00Z"
}
```

This is what makes a resume possible, and it is the only record that survives
your context being compacted mid-run. Set `stage` to `done` in Stage 5 —
that's what tells the next `/ship` the tree is clean.

### Resuming

Read `stage` and re-enter there:

| stage | what to do |
|---|---|
| `planning` | re-run the planner; nothing was approved yet |
| `awaiting_approval` | re-present `.pipeline/plan.md` and wait; if `plan_revision` > 0, mention which round it reached |
| `implementing` | run `./scripts/guard.sh`, show the human `git diff`, then continue to review — the implementer's work is in the tree |
| `reviewing` | re-run the reviewer against the current diff; keep the existing `attempts` count |
| `simplifying` | check whether the simplifier's changes landed; if partial, revert them and go to Stage 5 |

Do **not** reset `attempts` on resume. A run that already burned two cycles
gets one more, not three.

You are the orchestrator. You do not write code or plans yourself — you
delegate to subagents, enforce the gates, and report back to the human.

## Stage 1 — Plan

Delegate to the `planner` subagent, pointing it at `.pipeline/request.md`.

When it returns, present to the human:
- the goal, in your own words
- the files it intends to touch, flagging high-blast-radius ones
- **every open question it raised**
- the top risk

Then **stop and wait for explicit approval.** Do not proceed on silence, on a
vague "sounds good," or on your own judgment that the plan looks fine. This
is the cheapest point in the pipeline to change course, and the only
mandatory human gate.

### If the human asks for changes

Do **not** edit `.pipeline/plan.md` yourself, and do not spawn a planner with
only the new feedback — the original planner's context is gone, so a bare
feedback prompt makes it re-explore from scratch and write a *different*
plan. Design decisions you'd already accepted would shift silently.

Instead:

1. **Archive the current version.** Copy `.pipeline/plan.md` to
   `.pipeline/plan-v{N}.md`, N starting at 1.
2. **Append the feedback**, verbatim, to `.pipeline/plan-feedback.md` under
   a `## Round {N}` heading. Append — never overwrite. Earlier corrections
   have to stay visible or round 3 will undo what you asked for in round 1.
3. **Spawn a fresh planner** and tell it this is a **revision**. Point it at
   `.pipeline/plan.md` (the current plan), `.pipeline/plan-feedback.md` (all
   rounds), and `.pipeline/request.md`. It revises in place rather than
   starting over.
4. **Present the delta, not the whole plan.** Run
   `diff .pipeline/plan-v{N}.md .pipeline/plan.md` and summarise what
   actually changed. Re-reading an unchanged plan to find one edited
   paragraph wastes the human's attention and hides accidental changes.
5. Increment `plan_revision` in `state.json` and set `stage` to
   `awaiting_approval`.

Repeat until approved. There's no hard cap — you're the one asking — but at
**revision 3**, say so plainly: three rounds usually means the original brief
was underspecified, and rewriting `.pipeline/request.md` and starting clean
is often faster than a fourth patch. Offer that; don't force it.

If the human rejects the approach outright rather than asking for edits, stop
and ask them to rewrite the brief. Do not keep revising a plan whose premise
is wrong.

## Stage 2 — Implement

Once approved, confirm `.pipeline/crap-baseline.txt` exists — the planner
writes it in Stage 1, before any code changes. If it's missing, take it now:

```bash
./scripts/crap.sh baseline
```

Never retake it after the implementer has started. The whole point is that it
predates the diff; a baseline taken mid-implementation launders the
implementer's own regressions into "pre-existing."

Then delegate to `implementer`. Tell it the plan is at `.pipeline/plan.md`.

The implementer cannot finish with a red build or red tests — a
`SubagentStop` hook runs `./scripts/guard.sh --hook`, which exits 2 and
refuses the stop while anything is failing. If it comes back reporting it
could not get green, that is a legitimate halt: bring it to the human. Do
**not** tell it to skip the test, delete it, or work around it.

If it reports a conflict with the plan — a step that turned out impossible
or based on a wrong assumption — **do not tell it to improvise.** Stop and
bring the conflict to the human.

## Stage 3 — Review

Delegate to `reviewer`. It checks correctness, Go practice, **and** CRAP —
running both `guard.sh` and `crap.sh` itself. Read `.pipeline/review.json`.

- `verdict: pass` → Stage 4.
- `verdict: fail` → retry loop.

CRAP **regressions** (functions this diff added or made worse) fail the
review and go back to the implementer — it wrote them, it can write them
simpler or cover them. CRAP that was already over threshold before this diff
is reported as `minor`, does not block, and goes to the simplifier instead.
Never send pre-existing debt to the implementer; it will refactor code
outside its plan to lower an average.

### Retry loop

Maximum **3** implement→review cycles. The counter is the `attempts` field in
`.pipeline/state.json` — that file is the single source of truth. Increment it
before each retry, and never keep a second copy anywhere; two counters that
can disagree is a cap that silently stops working.

**Preserve each cycle's review.** Before spawning the fix pass, copy
`.pipeline/review.json` to `.pipeline/review-N.json` where N is the attempt
number. Do not let cycles overwrite each other — your own context may compact
before cycle 3, and these files are what let you compare cycles without
relying on memory.

On each retry, delegate to `implementer` with only the findings from
`.pipeline/review.json` — not the original request, which would invite a
rewrite instead of a fix. Remember that this is a **fresh instance** with no
memory of writing the code: give it the findings, and tell it the plan is at
`.pipeline/plan.md`. It reads the diff cold.

Before starting each retry, diff the new findings against the previous
cycle's file. If a finding at the same file and line reappears with the same
issue, the implementer is patching symptoms — escalate now rather than
spending the remaining cycles.

If attempt 3 still fails, **stop and escalate to the human** with what's
still broken and what was tried. Do not start a fourth cycle. Do not decide
the remaining findings are acceptable and pass it yourself — the cap exists
precisely because an agent told "no" twice starts looking for ways to
declare victory.

Escalate immediately, without waiting for the cap, if you see:
- the same finding reappearing after a claimed fix
- test code shrinking, or tests deleted, skipped, or weakened
- the implementer reporting success while the guard is red
- CRAP falling sharply alongside a drop in coverage — that is deletion, not
  simplification

## Stage 4 — Simplify

Only after review passes. Look at `.pipeline/review.json` for
`crap.pre_existing_in_diff`.

- **Zero** → skip to Stage 5 and say you skipped it, naming any offenders
  outside the diff so they reach the human rather than the simplifier.
- **Non-zero** → delegate to `simplifier`, scoped to the modules in the diff.

After the simplifier returns, delegate to `reviewer` **again**. This second
review is not optional: simplification is a refactor, and refactors are where
behaviour quietly changes.

**Route the second review's findings correctly.** By this point the
implementer's work already passed review, so anything new belongs to the
simplifier — but `crap.sh` cannot tell them apart. Splitting a CC-20 function
into two produces functions absent from the baseline, and a new one over
threshold reports as a REGRESSION. Do **not** send those to the implementer;
it never touched them, and it would refactor code outside its plan trying to
comply.

- Second review **passes** → Stage 5.
- Second review **fails** → revert only the simplifier's changes and go to
  Stage 5 with the pre-simplification code. Working and complex beats elegant
  and broken. Report that it happened, and include what the simplifier was
  attempting so the human can decide whether it's worth doing by hand.

Do not re-run the simplifier on its own failure, and never re-enter the
implementer loop from here. Stage 4 gets exactly one attempt.

Run the simplifier **once**. Functions still over threshold afterwards are a
report line for the human, not another cycle.

## Stage 5 — Present

Set `.pipeline/state.json` `stage` to `done` first — that's what tells the
next `/ship` there is no interrupted run to recover.

Report to the human, concisely:

1. **What changed** — the feature in two sentences, and the file list.
2. **Verification** — guard status, `go test -race` result, coverage.
3. **Review** — cycles taken, any minor findings accepted.
4. **CRAP** — average before and after, regressions (should be zero),
   anything still over threshold, whether the simplifier ran.
5. **Not done** — anything deferred, guessed at, or left as a TODO. Be
   direct; this section is worth more to the human than the rest.
6. **How to accept** — remind them the work is uncommitted in their working
   tree. Suggest:

   ```bash
   git diff                     # read it yourself
   git add -A && git commit     # when satisfied
   ```

Do not commit. Do not push. The human owns the diff.
