# Agent delivery pipeline for Go — start here

> **This describes installing the pipeline into another Go repo.** In envprobe
> it is already installed: `.claude/` and `scripts/` are tracked, and the
> `CHECKS` array in `scripts/guard.sh` is already set. Skip the install steps.

A `/ship` command for Claude Code that runs **plan → implement → review →
simplify → present**, with gates between stages and a mandatory human
approval after the plan.

Built for a Go CLI project on macOS. Scripts are bash 3.2 / BSD safe.

---

## Install

```bash
unzip claude-pipeline.zip
cd claude-pipeline
./install.sh /path/to/your/repo
```

Safe to re-run. It never overwrites an existing `.claude/settings.json` or a
customised `scripts/guard.sh` — it writes `*.pipeline-new` beside them for you
to merge.

**Note the folder is named `dot-claude/` in this bundle**, not `.claude/` —
a leading dot would make it invisible in Finder, and people reasonably
conclude the agents are missing. `install.sh` renames it to `.claude/` in
your repo, which is what Claude Code looks for.

Copying by hand instead:

```bash
cp -r dot-claude /path/to/your/repo/.claude     # note the rename
cp -r scripts    /path/to/your/repo/
chmod +x /path/to/your/repo/scripts/*.sh
```

---

## Then three things

**1. Edit the `CHECKS` array in `scripts/guard.sh`.** Nothing works until you
do. List your own check scripts in order — anything needing a built binary
comes after the build step:

```bash
CHECKS=(
  "./scripts/check.sh"
  "go build -o ./bin/app ./cmd/app"
  "./scripts/exit-codes.sh"
)
```

**2. Verify it runs.**

```bash
./scripts/guard.sh
```

Each check numbered `[n/N]`, then gate-integrity, placeholder, erosion, and
secret sections.

**3. Restart Claude Code.** A running session won't detect a `.claude/agents/`
directory that didn't exist when it started.

---

## Use it

On a clean working tree — every gate diffs against `HEAD`, so commit or stash
first:

```
/ship add a --limit flag to the list command
/ship specs/rate-limit.md          # a brief file, for anything structured
/ship                              # resume, or reuse the last brief
```

You approve the plan before any code is written. At the end the work sits
**uncommitted in your working tree** — read `git diff`, then commit yourself.
Nothing is ever committed or pushed for you.

Start with something small and boring. The first run tests the pipeline, not
the feature.

---

## What's in here

| File | |
|---|---|
| `install.sh` | copies everything into your repo |
| `dot-claude/commands/ship.md` | the orchestrator — gates, retry cap, routing |
| `dot-claude/agents/*.md` | **the four agents** — planner, implementer, reviewer, simplifier |
| `dot-claude/settings.json` | deny-list, gofmt hook, mandatory-gate hook |
| `scripts/guard.sh` | runs your checks + 5 agent-specific ones |
| `scripts/crap.sh` | CRAP gate via crap4go, with baseline comparison |
| `docs/pipeline-INTEGRATION.md` | **full guide** — setup, accepting work, resuming, macOS notes |
| `docs/pipeline-README.md` | design rationale — why each gate exists |

---

## Verify on your first run

**The `SubagentStop` matcher.** `settings.json` scopes the mandatory gate to
`^implementer$`. If that doesn't filter by subagent name in your Claude Code
version, the hook fires after every subagent — including the reviewer, which
would then be unable to report a `fail` verdict. Symptom: the reviewer never
returns a verdict on a broken diff. Fix: delete the `SubagentStop` block; you
drop to one enforced gate instead of two.

Details in `docs/pipeline-INTEGRATION.md`.
