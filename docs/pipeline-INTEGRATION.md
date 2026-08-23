# Integrating into an existing Go project

Assumes: a Go module in git, a working check script, and Claude Code
installed.

---

## 1. Copy the files

Everything goes at your **repo root** — the directory holding `go.mod`. The
downloaded bundle has a `claude-pipeline/` wrapper folder; copy its *contents*
in, not the folder itself.

```
your-project/
├── go.mod
├── check.sh                    ← your existing script, untouched
├── .claude/
│   ├── settings.json           ← merge if you already have one
│   ├── agents/
│   │   ├── planner.md
│   │   ├── implementer.md
│   │   ├── reviewer.md
│   │   └── simplifier.md
│   └── commands/
│       └── ship.md
└── scripts/
    ├── guard.sh
    └── crap.sh
```

```bash
./install.sh /path/to/your/repo     # does all of this for you
```

By hand — note the bundle ships `dot-claude/`, which must be renamed to
`.claude/` in your repo. The bundle avoids the leading dot only so the folder
is visible in Finder:

```bash
cp -r claude-pipeline/dot-claude /path/to/your/repo/.claude
cp -r claude-pipeline/scripts    /path/to/your/repo/
chmod +x /path/to/your/repo/scripts/*.sh
```

**If you already have `.claude/settings.json`**, merge rather than overwrite —
you'd lose your existing config. The pipeline adds three keys: `skills`,
`permissions.deny`, and `hooks`. If you already have hooks, append to the
arrays; `SubagentStop` and `PostToolUse` can hold multiple entries.

Check these in. They're project configuration, and the point is that your
whole team gets the same gates.

`pipeline-README.md` and `pipeline-INTEGRATION.md` are documentation, not
runtime files — they live under `docs/` here, and can be discarded if you
don't want them.

---

## 2. Wire up your check scripts

`guard.sh` does not replace your scripts — it runs them, then adds five
agent-specific checks on top.

Edit the two arrays at the top of `guard.sh`. For a CLI app with unit checks
plus binary-level scenario tests, the chain is:

```bash
CHECKS=(
  "./scripts/check.sh"                  # fmt, vet, golangci, tests, coverage
  "go build -o ./bin/app ./cmd/app"     # exit-codes.sh needs the binary
  "./scripts/exit-codes.sh"             # scenario + exit-code validation
)

CHECKS_REVIEW_ONLY=(
)
```

Adjust the build line to your actual binary name and `cmd/` path.

**Order matters and the chain is fail-fast.** It stops at the first failure
and tells the agent which checks it never reached:

```
=== [1/3] ./scripts/check.sh ===
  FAIL: ./scripts/check.sh
  Chain stopped. Remaining 2 check(s) not run:
    - go build -o ./bin/app ./cmd/app
    - ./scripts/exit-codes.sh
```

That's deliberate. Running `exit-codes.sh` against a stale binary from three
edits ago produces results that are worse than useless — they're
confidently wrong, and the agent will chase them.

The explicit build step matters for the same reason. `check.sh` compiles
packages during `go test`, but that doesn't refresh `./bin/app`. Without the
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
those: `bash -c 'CGO_ENABLED=0 go build -o ./bin/app ./cmd/app'`.

**Start every run from a clean tree.** Commit or stash first. Every gate
diffs against `HEAD`, so uncommitted work gets attributed to the implementer
— and the CRAP baseline would launder it into "pre-existing". `/ship` checks
`git status --porcelain` and stops if the tree is dirty.

Verify before going further:

```bash
./scripts/guard.sh
```

You want each check numbered `[n/N]` and passing, then the placeholder,
erosion, and secret sections.

Two things to confirm about your own scripts:

- **They exit non-zero on failure.** If `exit-codes.sh` prints a mismatch but
  exits 0, the mandatory gate is decorative and the implementer can hand back
  a binary with wrong exit codes.
- **Add your build output to `.gitignore`** (`bin/`). Untracked build
  artifacts otherwise show up in `git status` and muddy the review.

`CHECK_CMD='./scripts/check.sh' ./scripts/guard.sh` overrides the whole chain
for a quick manual run.

---

## 3. Install crap4go

```bash
go install github.com/unclebob/crap4go/cmd/crap4go@latest
```

`crap.sh` installs it automatically on first run if missing, but doing it now
surfaces PATH problems while you're paying attention.

Add the skill too, so the reviewer and simplifier get the methodology in
context and not just a table of numbers. It's already in the bundled
`settings.json`:

```json
{ "skills": ["https://github.com/unclebob/crap4go/blob/master/SKILL.md"] }
```

Then see your numbers:

```bash
./scripts/crap.sh baseline
cat .pipeline/crap-baseline.txt
```

**Expect this to be ugly on an existing codebase.** That's fine and it does
not block anything — the baseline mechanism means only functions *this diff*
makes worse will ever fail a review. Legacy debt is reported every run and
routed to the simplifier, never to the implementer.

If the numbers are catastrophic and you want a gentler start, raise the
threshold and walk it down over time:

```bash
# .claude/settings.json or your shell profile
CRAP_THRESHOLD=25 CRAP_WARN=15
```

---

## 3b. CRAP and your `cmd/` package

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

## 4. gitignore

```bash
printf '.pipeline/\ntarget/\nbin/\n' >> .gitignore
```

`.pipeline/` is pipeline working state. `target/` is where crap4go writes
coverage.

---

## 5. First run

Pick something small and boring for the first feature — a validation
function, a small endpoint. You are testing the pipeline, not the feature.

```
/ship add a MaxBodyBytes limit to the upload handler
```

Watch for four things:

1. **Does the planner ask real questions?** An empty "Open questions" list on
   a genuine feature usually means it invented requirements instead of
   surfacing ambiguity.
2. **Does the approval gate actually stop?** It should present the plan and
   wait. If it barrels into implementation, the command file isn't loading —
   restart Claude Code.
3. **Does the reviewer find anything?** A clean first-cycle pass every time
   means the reviewer prompt is too soft for your codebase.
4. **Is the "Not done" section ever non-empty?** Real work has loose ends. An
   always-empty section means it's being papered over.

If a subagent doesn't exist, restart — a running session won't pick up a
`.claude/agents/` directory created after it started.

---

## 5b. Revising a plan

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

## 6. Accepting a finished feature

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

## 7. If a run stops partway

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

## 8. Adoption path

Don't turn everything on at once.

**Week 1 — planning only.** Run `/ship`, approve or reject plans, then stop
and implement by hand. You'll learn fast whether the planner understands your
codebase, and it costs almost nothing to find out.

**Week 2 — add the gates.** Let it implement. Keep the retry cap at 3 and
watch what happens when the reviewer pushes back. This is where you find out
whether the guard is catching real problems or generating noise.

**Week 3 — tune the reviewer.** Add findings it missed to `reviewer.md` as
explicit hunt items. This file should drift toward your codebase's actual
failure modes — the ones you keep catching in human review are exactly what
belongs there.

**Later — CRAP tightening.** Once regressions are consistently zero, lower
`CRAP_THRESHOLD` and let the simplifier start working through legacy debt.

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

**2. The crap4go skill URL — verified.**

`settings.json` points at
`https://github.com/unclebob/crap4go/blob/master/SKILL.md`. That file exists
and `master` is the repo's default branch, so the `skills` entry resolves.

If it ever 404s, drop the `skills` array. Nothing breaks — `crap.sh` shells
out to the `crap4go` binary regardless, and the skill only adds methodology
context for the reviewer and simplifier.

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| "no checks configured" | `CHECKS` array is empty — edit `guard.sh` |
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
