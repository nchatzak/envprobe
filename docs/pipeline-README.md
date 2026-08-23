# Agent delivery pipeline (Go)

Plan → implement → review → simplify → present, with gates between stages.

## Install

Copy into your repo root and check it in:

```
.claude/agents/planner.md
.claude/agents/implementer.md
.claude/agents/reviewer.md
.claude/agents/simplifier.md
.claude/commands/ship.md
.claude/settings.json
scripts/guard.sh
scripts/crap.sh
```

Then:

```bash
chmod +x scripts/guard.sh scripts/crap.sh
echo ".pipeline/" >> .gitignore
```

Point the guard at your check scripts by editing the `CHECKS` array at the
top of `guard.sh`. There is no auto-detection — an empty array is a setup
error and guard exits 2 telling you to fill it in.

For a one-off manual run, `CHECK_CMD` replaces the whole chain with a single
command:

```bash
CHECK_CMD='./scripts/check.sh' ./scripts/guard.sh
```

`crap.sh` installs crap4go automatically on first run. To do it yourself:

```bash
go install github.com/unclebob/crap4go/cmd/crap4go@latest
```

crap4go runs the suite itself and writes to `target/coverage/coverage.out` —
it won't reuse your check script's profile. If that double run bothers you,
narrow it:

```bash
CRAP_TEST_CMD="go test ./internal/app" ./scripts/crap.sh
```

Restart Claude Code — a running session doesn't detect a `.claude/agents/`
directory that didn't exist when it started.

## Run

Inline, for something small:

```
/ship add rate limiting to the public HTTP handlers
```

Long text works too — a full paragraph is fine, `$ARGUMENTS` takes all of it.

For anything with real structure, write a brief and pass the path:

```
/ship specs/rate-limit.md
/ship @specs/rate-limit.md
```

A brief is worth it once you have acceptance criteria, example payloads,
API shapes, or ticket links — things that lose meaning compressed into a
sentence. There's no required format; the planner reads it as prose. Whatever
you pass gets copied verbatim to `.pipeline/request.md`, and every stage
reads it from there, so files and inline text behave identically after that.

`/ship` with no argument falls back to `.pipeline/request.md` if it exists,
which is the convenient form when iterating on the same brief.

## The flow

```
  you ──▶ planner ──▶ .pipeline/plan.md
                          │
                    ┌─────▼──────┐
                    │ YOU APPROVE │  ◀── the one mandatory human gate
                    └─────┬──────┘
                          ▼
   planner also writes crap-baseline    ◀── snapshot BEFORE any changes
                          │
                    implementer ──▶ diff
                          │
                   guard.sh --hook       ◀── exit 2 blocks the stop.
                          │                  cannot finish red.
                       reviewer
                    correctness +
                    practice + CRAP ──▶ .pipeline/review.json
                          │
                    pass ─┴─ fail ──▶ implementer (max 3, then escalate)
                     │                 (regressions only, never legacy debt)
                     ▼
              pre-existing CRAP?
               │            │
             yes            no
               │            │
          simplifier        │
               │            │
           reviewer ────────┤   re-review after refactor, not optional
                            ▼
                         present
```

## The guards

**Two gates, at different speeds.**

`guard.sh` is the **mandatory** one. It runs your check scripts — an ordered,
fail-fast chain you define in the `CHECKS` array, so build steps can precede
the scripts that depend on them — then adds five agent-specific checks. A
`SubagentStop` hook runs it as `--hook`, which exits
**2** — that's the code that actually blocks a subagent from stopping and
feeds the failure back to it. The implementer cannot hand back a red build.
(Exit 1 would not have blocked anything; the hook only refuses on 2.)

| Added check | Why your check script doesn't have it |
|---|---|
| gate integrity | an agent that edits the gate judging it is not passing, it is escaping |
| scenario erosion | deleted CLI scenarios and fixtures don't show up in test line counts |
| placeholder scan | a human doesn't ship `panic("not implemented")` and call it done |
| **test erosion** | a human doesn't delete the test that's blocking them |
| secret scan | cheap, and agents paste example credentials |

Test erosion behaves differently by mode, deliberately. In `--hook` mode it
**warns**; at review it **fails**. If it blocked at the hook, an implementer
whose test refactor legitimately shrank the line count could neither finish
nor explain itself — and the only escape it could find would be padding tests
with filler until the delta went positive. Shrinking tests is a judgment
call, so it belongs where it can reach you.

CRAP is deliberately **not** in `guard.sh` — crap4go re-runs the whole suite,
which is too slow to fire on every implementer stop. It runs once, at review.

`crap.sh` is the **review-time** gate, and the reviewer owns it. Complexity
and missing coverage are review findings, not a separate metric bolted on:
a correct, idiomatic, CC-18 function at 30% coverage is a liability, and
catching that is the reviewer's job alongside correctness and Go practice.

### Threshold: 15, with a baseline

The conventional threshold is 30. That's too loose for Go, where functions
should be small — and remember that at 100% coverage CRAP equals CC, so a
threshold of 30 permits a fully-tested CC-29 function. Default here is **15**
to fail, **10** to warn.

A tight threshold on a real codebase would normally be unusable — everything
lights up on day one. The baseline fixes that:

```bash
./scripts/crap.sh baseline    # snapshot, taken before implementation
./scripts/crap.sh             # score and compare
```

The orchestrator snapshots automatically after you approve the plan. Then:

| | Severity | Goes to |
|---|---|---|
| **REGRESSION** — function added or made worse by this diff | `major`, blocks | implementer |
| **PRE-EXISTING** — already over threshold before the diff | `minor`, reports | simplifier |
| **N/A** — no coverage mapped | `major` | implementer (untested) |

That split is what makes 15 adoptable. Your legacy CC-20 monster gets
reported every run and never blocks anything; a new function at CRAP 55 fails
immediately. It also stops the implementer refactoring code outside its plan
to pull an average down.

**Which lever to pull** depends on the numbers, and the reviewer is told to
check rather than guess: a CC-9 function at 20% coverage scores 55 and needs
*tests* (it'd be 9 fully covered). A CC-22 function at 95% needs *splitting* —
no coverage saves it.

```
CRAP(f) = CC(f)² × (1 − cov(f))³ + CC(f)
```

The metric is Alberto Savoia's, from crap4j (Agitar, 2007).

**crap4go is both a CLI and a Claude Code skill.**

As a CLI:

```bash
go install github.com/unclebob/crap4go/cmd/crap4go@latest
crap4go                              # whole module
crap4go internal/combat movement     # filter by path fragments
```

As a skill — add to `.claude/settings.json` (note: branch is `master`):

```json
{ "skills": ["https://github.com/unclebob/crap4go/blob/master/SKILL.md"] }
```

Then ask for a "CRAP report." Install both: the skill gives the simplifier
the methodology in context, the CLI gives `guard.sh` something to gate on.

### The gotcha: no --threshold flag

crap4go prints a report and exits 0. It is a *reporting* tool, not a gate.
`scripts/crap.sh` runs it, parses the table, and applies the threshold — that
parsing is the reason the script exists at all. If you were planning to wire
`crap4go` straight into CI expecting a non-zero exit, it won't do that.

`crap.sh` installs crap4go if it isn't present (`go install
.../cmd/crap4go@latest`) and prepends `go env GOBIN` to PATH for the run,
falling back to `$(go env GOPATH)/bin` when `GOBIN` is unset.
There is no fallback path — if crap4go can't be installed or run, the gate
exits 2 and the pipeline stops rather than silently scoring nothing.

### It runs your tests itself

crap4go deletes stale coverage, then runs:

```
go test ./... -coverprofile=target/coverage/coverage.out
```

It does not accept an externally generated profile, so it **cannot reuse the
coverage your check script already produced** — running it after your check
script means the suite runs twice. Narrow it with `--test-command`:

```bash
CRAP_TEST_CMD="go test -tags appunit ./internal/app" ./scripts/crap.sh
```

crap4go appends the coverprofile flag, or substitutes `{coverprofile}` if you
include it. `--max-workers` controls parallelism (defaults to half your
logical CPUs).

Add `target/` to `.gitignore` alongside `.pipeline/`.

### What counts toward CC

`if`, `for`, `range`, each `switch` and type-switch case clause, each
`select` communication clause, `&&`, `||`. Note `for` and `range` both
count — loop-heavy Go scores higher than you might expect.

Skipped: `_test.go`, `target/`, `vendor/`, `.git/`.

### Risk bands

| CRAP | Risk |
|---|---|
| 1–5 | low — clean code |
| 5–30 | moderate — refactor or add tests |
| 30+ | high — complex and under-tested |

Functions reported `N/A` have no coverage mapped. `crap.sh` counts them
separately and does not gate on them — but N/A means *unmeasured*, not clean.

Two properties worth internalising:

- At 100% coverage the quadratic term collapses and **CRAP equals CC**. A
  fully-tested function still carries its complexity.
- Above CC ≈ 30, **no amount of coverage** gets you under threshold. The
  formula is saying the function is too big to certify regardless of testing.

But note the order that follows from this — **tests first, then refactor**,
which is Uncle Bob's documented loop and the opposite of what I'd guess:

1. CRAP report → pick the worst function in the module you're changing
2. **Add characterization tests** until coverage is high enough to change
   the code safely
3. *Then* refactor the complex branches
4. Re-run CRAP and the suite; move to the next function

A function scores 130 because it's complex *and* under-covered. Refactoring
it at 45% coverage means you have no way to know you broke it. And since
`(1-cov)³` collapses fast, step 2 often drops it under threshold on its own —
CC 12 at 45% scores 130; CC 12 at 90% scores 12.1. Check the score between
the two steps; sometimes step 3 turns out to be unnecessary.

Characterization tests pin what the function *currently does*, including the
behaviour you think is wrong. They're a safety net for the refactor, not a
correctness statement.

## How the agents communicate

They don't — not with each other. Subagents can't call other subagents and
can't see each other's context. Each one is spawned into a fresh context
window, does its work, returns **one final summary**, and is gone.

The orchestrator (`ship.md`, in your main session) is the only thread that
persists. It reads the returned summaries, reads the files, and decides what
to spawn next. Every routing rule — the retry cap, the escalation triggers,
the regression-vs-legacy split — is enforceable only there.

Two channels, deliberately:

| Channel | Carries | Why |
|---|---|---|
| Return summary | verdict, counts, top finding | small; lands in orchestrator context for routing |
| `.pipeline/*` files | plans, findings, baselines | durable, inspectable, survives compaction |

If everything went through summaries, the orchestrator's context would be
full of code by cycle two. If everything went through files, it would have
nothing to route on without reading them every turn.

**Each cycle spawns a fresh implementer.** The one fixing review findings has
no memory of writing the code — it reads `plan.md`, `review.json`, and the
diff cold. That's mostly a feature: it can't get defensive about its own
work or rationalise a finding away by recalling why it seemed reasonable at
the time. The cost is that it doesn't know *why* a choice was made, so it can
"fix" a finding by undoing something deliberate. The second review is what
catches that.

Reviews are preserved per cycle as `.pipeline/review-1.json`,
`review-2.json`, and so on, so the "same finding came back" check compares
files rather than trusting the orchestrator to remember across a compaction.

## Where "simplify" goes

Three different things share that word. Keeping them separate matters:

**1. `gofmt -s` — mechanical.** Deterministic rewrites, no judgment. A gate
condition plus a `PostToolUse` hook, so it never becomes a finding.

**2. `code-simplifier` — the official Anthropic plugin.** It exists:

```
/plugin install code-simplifier@claude-plugins-official
```

It's a subagent that refines recently-modified code for clarity while
preserving functionality. Good tool, but **not a drop-in for this pipeline's
stage**, for three reasons:

- Its coding-standards section is JavaScript/TypeScript specific — ES
  modules, `function` over arrow functions, React Props types. None of that
  applies to Go, and it's baked into the agent prompt.
- It's built to fire **autonomously and proactively, immediately after code
  is written**. That's the exact ordering this pipeline forbids: simplifying
  before review makes "I simplified it" available as a reason a branch
  disappeared. Here, simplification runs *after* the reviewer passes.
- It has no CRAP targeting and no instruction to run tests. It aims for
  clarity in general; the stage here aims at specific functions over
  threshold, and verifies by re-running the suite.

Use it ad-hoc if you like it. For the pipeline stage, `simplifier.md` is
essentially the same idea specialised to Go, CRAP, and the after-review slot.

**3. `simplifier.md` — the pipeline stage.** Structural, CRAP-targeted, runs
after review passes, capped at one pass and three functions, followed by a
mandatory second review.

That ordering is the load-bearing part. By the time the simplifier runs, the
tests are the frozen contract — which is why it may add characterization
tests but never edit existing ones.

## Why it's shaped this way

**State lives in files, not chat history.** Each agent starts with a fresh
context window and reads `.pipeline/plan.md` or `.pipeline/review.json`. When
output is wrong you can open the plan and see whether the planner or the
implementer caused it. A crashed run resumes instead of restarting.

**The reviewer returns JSON.** You can branch on `verdict == "fail"`. You
cannot reliably branch on three paragraphs of tactful feedback.

**Tool access is narrowed per agent.** The planner can't edit code. The
reviewer can't fix what it finds. This isn't only safety — it stops an agent
quietly resolving a problem it should have escalated.

**The retry loop is capped at 3.** Uncapped review loops don't converge; they
burn tokens while the implementer looks for ways to satisfy the reviewer,
including deleting tests. The cap turns a runaway into an escalation.

## Tuning

**Cost.** Set `model: sonnet` on the implementer once it behaves; keep
`opus` on planner, reviewer, and simplifier where judgment matters.

**Threshold.** `CRAP_THRESHOLD=10 CRAP_WARN=7 ./scripts/crap.sh` to tighten
further. The baseline mechanism means a tight threshold is safe to adopt on
day one — legacy debt is reported, never blocking.

**Isolation.** Add `isolation: worktree` to the implementer's frontmatter to
have it work in a temporary git worktree instead of your checkout.

**Parallel review.** Split `reviewer` into `reviewer-concurrency` and
`reviewer-correctness` and run both, then merge findings. Two narrow
reviewers catch more than one broad one — and Go concurrency review is
specialised enough to deserve its own prompt.

**Non-interactive.** Once stable: `claude -p "/ship $FEATURE"` in CI.

## Failure modes to watch for

The pipeline is working when it tells you bad news. Be suspicious when:

- reviews always pass first cycle — the reviewer prompt is too soft
- the "Not done" section is always empty — real work has loose ends
- the same finding recurs across cycles — the implementer is patching the
  symptom the reviewer named, not the cause
- the diff shrinks between cycles — check whether tests were removed
- CRAP drops sharply after simplification — verify coverage didn't drop
  alongside it; deleting a branch lowers both CC and the tests that covered it

Read the diff yourself before committing. The pipeline raises the floor on
quality; it doesn't replace you as the last reviewer.
