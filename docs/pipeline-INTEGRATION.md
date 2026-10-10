# Running the pipeline

## 1. The check chain

`guard.sh` does not replace envprobe's scripts — it runs them, then adds five
agent-specific checks on top.

The chain is the two arrays at the top of `guard.sh`:

```bash
CHECKS=(
  "./scripts/check.sh"        # fmt, vet, golangci, tests, coverage
  "go build -o envprobe ."    # exit-codes.sh needs the binary
  "./scripts/exit-codes.sh"   # scenario + exit-code validation
)

CHECKS_REVIEW_ONLY=(
)
```

**Order matters and the chain is fail-fast.** It stops at the first failure
and tells the agent which checks it never reached:

```
=== [1/3] ./scripts/check.sh ===
  FAIL: ./scripts/check.sh
  Chain stopped. Remaining 2 check(s) not run:
    - go build -o envprobe .
    - ./scripts/exit-codes.sh
```

That's deliberate. Running `exit-codes.sh` against a stale binary from three
edits ago produces results that are worse than useless — they're
confidently wrong, and the agent will chase them.

The explicit build step matters for the same reason. `check.sh` compiles
packages during `go test`, but that doesn't refresh `./envprobe`. Without the
build entry, `exit-codes.sh` validates whatever binary happened to be lying
around.

**Two tiers.** `CHECKS` runs everywhere. `CHECKS_REVIEW_ONLY` is skipped in
`--hook` mode, which fires on every implementer stop — possibly several times
per feature. Start with everything in `CHECKS`; the implementer catching a
broken exit code itself is much better than burning a review cycle on it.
Only demote to `CHECKS_REVIEW_ONLY` if you measure a check being slow enough
to hurt:

```bash
time ./scripts/exit-codes.sh
```

Under ~15s, leave it in `CHECKS`. Well over, demote it and accept that
scenario failures cost a review cycle instead of being caught immediately.

**Setup errors are distinguished from code failures.** If a configured check
isn't executable or isn't on PATH, guard exits 2 with a message naming the
entry to fix, rather than reporting your code as broken.

One wrinkle: the check reads the *first word* of the entry, so an inline env
prefix (`CGO_ENABLED=0 go build ...`) looks like a missing executable. Wrap
those: `bash -c 'CGO_ENABLED=0 go build -o envprobe .'`.

**Start every run from a clean tree.** Commit or stash first. Every gate
diffs against `HEAD`, so uncommitted work gets attributed to the implementer
— and the CRAP baseline would launder it into "pre-existing". `/ship` checks
`git status --porcelain` and stops if the tree is dirty.

`CHECK_CMD='./scripts/check.sh' ./scripts/guard.sh` overrides the whole chain
for a quick manual run.

---

## 2. crap4go

`crap.sh` installs crap4go on first run if it is missing. To see the
numbers:

```bash
./scripts/crap.sh baseline
cat .pipeline/crap-baseline.txt
```

Functions already over threshold do not block anything — the baseline
mechanism means only functions *this diff* makes worse will ever fail a
review. Legacy debt is reported every run and routed to the simplifier, never
to the implementer.

---

## 2b. CRAP and the `cmd/` package

Scenario coverage is invisible to crap4go — `exit-codes.sh` exercises the
built binary, crap4go reads a `go test` profile. So `cmd/` code shows near-0%
coverage even when scenarios exercise it thoroughly.

**Don't filter `cmd/` out of the gate.** Two things make this a non-issue
without any extra machinery.

Your existing `cmd/` code is captured in the baseline, so it's pre-existing
and never blocks. And for new code, run the numbers — at 0% coverage the
formula collapses to `CC² + CC`:

| CC | CRAP at 0% coverage | vs threshold 10 |
|---|---|---|
| 2 | 6 | passes |
| 3 | 12 | fails |
| 4 | 20 | fails |
| 6 | 42 | fails |

**At threshold 10 the margin is thin.** Only CC 1–2 clears the gate at zero
coverage, so a `cmd/` function with a couple of branches trips it. That is
tolerable when `cmd/` is covered by `go test` — check your own numbers with
`go test ./... -cover` before assuming this section applies to you. If `cmd/`
really is near-0%, either keep dispatch at CC 2 or raise the threshold.

So a `cmd/` function failing CRAP is the gate working correctly, and the fix
is the one you'd want regardless: move the logic into `internal/` where
`go test` reaches it, and leave `cmd/` as wiring. The gate quietly enforces
the package layout you already wanted.

If you genuinely need scenario coverage counted — a large existing `cmd/`
you're not restructuring — Go 1.20+ supports `go build -cover` with
`GOCOVERDIR`, merged via `go tool covdata textfmt` and fed to crap4go through
`CRAP_TEST_CMD`. It works, but it's a real chunk of plumbing. Try thin
dispatch first.

---

## 3. Revising a plan

You'll reject the first plan often, and that's the cheapest place to spend
your attention.

Each revision spawns a **fresh** planner — the original's context is gone
once it returns. To stop it re-deriving your codebase and drifting from
decisions you'd already accepted, it's handed three things: the current
`.pipeline/plan.md`, the accumulated `.pipeline/plan-feedback.md`, and the
original `.pipeline/request.md`. It revises in place rather than starting
over, and is told to change only what you asked about.

Feedback **accumulates**. Round 3 does not replace round 1 — the planner
reads every round, which is what stops revisions oscillating (you ask for
something simpler, then ask it to handle a concurrent case, and round 3
quietly undoes the simplification). If two rounds genuinely conflict, it asks
instead of picking.

You get a **diff**, not the whole plan again:

```
diff .pipeline/plan-v2.md .pipeline/plan.md
```

Re-reading an unchanged plan to find one edited paragraph is how accidental
changes slip through.

Old versions stay as `.pipeline/plan-v1.md`, `plan-v2.md`, so you can see how
the thinking moved.

**At revision 3 it will say so.** Not a block — three rounds usually means
the brief was underspecified, and rewriting `.pipeline/request.md` and
starting clean is often faster than a fourth patch. Which is the argument for
using a brief file rather than a one-liner when the feature has any real
shape to it.

---

## 4. Accepting a finished feature

**The work is uncommitted in your normal working tree.** Not a branch, not a
worktree, not a patch file — the files are simply edited in place, exactly as
if you'd written them.

Nothing was committed, because `settings.json` denies `Bash(git commit:*)`
and `Bash(git push:*)`. That's deliberate: the pipeline raises the floor on
quality, it does not replace you as the last reviewer.

So accepting is just:

```bash
git diff                    # read it — actually read it
go test ./...               # trust but verify
git add -A
git commit -m "add MaxBodyBytes limit to upload handler"
```

If you want a branch, make one first — `git switch -c feat/upload-limit`
before `/ship`, or after the fact, since nothing is committed either way.

**Rejecting** is equally simple, since nothing is staged:

```bash
git checkout -- .           # discard everything
git clean -fd               # plus any new files it created
```

Read `.pipeline/plan.md` before you throw it away — a bad result is usually a
bad plan, and the fix is often re-running with a better brief rather than
abandoning the approach.

**Partial acceptance:** `git add -p` works normally. The pipeline has no
opinion about your staging.

### Optional: worktree isolation

If you'd rather the implementer not touch your checkout — so you can keep
working while it runs — Claude Code supports an `isolation` frontmatter field
on subagents for git worktree isolation. Add it to `implementer.md`:

```yaml
isolation: worktree
```

Verify the exact accepted value against `code.claude.com/docs/en/sub-agents`
before relying on it; `isolation` is a documented field but I have not
confirmed its value set. Note the tradeoff: with isolation on, the finished
work is in a separate worktree and you merge it back, which is more
ceremony. Without it, `git diff` in your own checkout is the whole workflow.

---

## 5. If a run stops partway

Context limits, a crash, you closing the laptop, or an escalation you didn't
answer — any of these leave a half-finished run.

**It's recoverable.** The orchestrator writes `.pipeline/state.json` after
every stage transition, and the next `/ship` reads it *before* touching
anything. Finding an unfinished run, it stops and asks whether you want to
resume or abandon. It will not silently archive your work.

```
/ship
```

with no arguments is the convenient way back in — it reports the stage
reached, the attempts used, and what's sitting in `git status`.

Resuming does **not** reset the attempt counter. A run that already burned
two review cycles gets one more, not three.

The reason this matters more than it sounds: `.pipeline/crap-baseline.txt`
was captured *before* implementation started and cannot be regenerated. Take
it again now and the code has already changed, so that run's own regressions
get reclassified as pre-existing debt and stop blocking. Losing the baseline
silently disables the CRAP gate for that feature.

To throw one away deliberately, just tell it to abandon when asked. The old
files move to `.pipeline/prev/` rather than being deleted.

---

## macOS notes

macOS ships **bash 3.2** (2007) as `/bin/bash`, and BSD userland rather than
GNU. Both scripts are written for that, and it's the constraint that dictates
some of the more awkward-looking code:

- Every array expansion is length-guarded. In bash 3.2, `"${arr[@]}"` on an
  *empty* array is an unbound-variable error under `set -u` — bash 4.4 made
  it legal. An empty `CHECKS_REVIEW_ONLY=()`, the default, would otherwise
  crash on every Mac.
- No `mapfile`, `readarray`, `declare -A`, or `${var,,}` anywhere.
- No `sed -i`, which needs an argument on BSD (`sed -i ''`).
- `wc -c` output is whitespace-stripped before comparison — BSD pads it.
- Binary detection uses `grep -qI`, which behaves the same on GNU and BSD.

You do not need Homebrew bash. If you have it, the scripts still work.

**If you edit `guard.sh` or `crap.sh` yourself, commit before running
`/ship`.** Gate integrity flags any uncommitted change to those files —
correctly, since it can't tell your edit from an agent's. The clean-tree
check catches this first anyway.

---

## Verify on first run

One thing in this setup cannot be tested without a real run. Check it during
your first `/ship` — it has a bad failure mode.

**1. Does the `SubagentStop` matcher scope to the implementer?**

`settings.json` uses `"matcher": "^implementer$"` so `guard.sh --hook` only
fires when the implementer finishes. If that matcher doesn't filter by
subagent name in your Claude Code version, the hook fires after **every**
subagent — including the reviewer.

That would be bad, not merely wasteful: the reviewer's job is to report a
`fail` verdict on a red diff, and a hook exiting 2 would stop it from
finishing and reporting at all. Deadlock, in the one agent that must always
be able to speak.

Symptom: the reviewer never returns a verdict on a diff that has failures,
or you see guard output after a stage that isn't implementation.

Fallback if it misbehaves — drop the `SubagentStop` block entirely. You lose
the hard enforcement, but the implementer is still instructed to run
`./scripts/guard.sh` before finishing, and the reviewer runs it independently
and fails the review. You're back to one enforced gate instead of two.

**2. The crap4go skill entry — removed, it never loaded.**

`settings.json` used to carry
`{ "skills": ["https://github.com/unclebob/crap4go/blob/master/SKILL.md"] }`.
The URL resolves, which is what was originally checked, but that is not the
same as the entry working. `skills` is not a documented settings key, no
crap4go skill shows up in a session's available skills, and nothing lands in
`~/.claude/skills` or `.claude/skills`. The entry was inert.

Nothing was lost by dropping it. `crap.sh` shells out to the `crap4go` binary,
which is the only path the pipeline uses, and no subagent could have invoked a
skill in any case — none of the four grants a `Skill` tool.

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| "not executable or not on PATH" | a `CHECKS` entry is wrong; exit 2 means setup, not code |
| `exit-codes.sh` fails on correct code | stale binary — confirm the build step precedes it in `CHECKS` |
| Subagent not found | session started before `.claude/agents/` existed — restart |
| Implementer finishes with red tests | hook not firing; confirm `--hook` is in `settings.json` and `guard.sh` is executable |
| Everything flagged as a CRAP regression | baseline missing or taken too late — check `.pipeline/crap-baseline.txt` predates the diff |
| Reviewer always passes first try | prompt too soft; add your real failure modes to `reviewer.md` |
| Tests run twice per cycle | expected — crap4go runs its own; narrow with `CRAP_TEST_CMD` |
| Escalates immediately on a fresh feature | stale `state.json` from a prior run; the setup step should have archived it |
| Refuses to start | working tree is dirty — commit or stash first; every gate diffs against HEAD |
| Gate integrity fails on a file you edited | commit your `guard.sh`/`crap.sh` changes first |
| `CHECKS` entry with an env prefix reported as setup error | the executable check reads the first word; wrap it: `bash -c 'CGO_ENABLED=0 go build ...'` |

---

## What is and isn't actually enforced

Worth being clear about, because it shapes how much you should trust a green
result.

**Enforced by the runtime** — the model cannot talk its way past these:

- `SubagentStop` hook exiting 2 (implementer cannot finish red)
- `permissions.deny` (no commits, no pushes, no `.env` reads)
- `maxTurns` caps in each agent's frontmatter
- `crap.sh` and `guard.sh` exit codes

**Instructions only** — an LLM following written rules, which mostly works
and occasionally doesn't:

- the 3-cycle retry cap
- "escalate, don't declare victory"
- the plan approval gate
- the regression-vs-legacy CRAP routing

Watch the orchestration layer specifically during your first few runs. If it
starts rationalising findings as acceptable rather than escalating, tighten
the language in `ship.md` — that file is the only place those rules live.
