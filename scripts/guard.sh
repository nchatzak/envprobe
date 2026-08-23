#!/usr/bin/env bash
# Mandatory gate. The implementer cannot finish with this red.
#
# CRAP is deliberately NOT here — it re-runs the whole suite and belongs to
# the review stage. This script must stay fast enough to run on every
# implementer stop.
#
# Usage:
#   ./scripts/guard.sh          exit 1 on failure (CI / manual)
#   ./scripts/guard.sh --hook   exit 2 on failure (blocks SubagentStop and
#                               feeds the failure back to the agent)
#
# Env:
#   CHECK_CMD   overrides the CHECKS chain entirely, for one-off manual runs

# ============================================================================
# YOUR CHECKS — edit these two arrays.
# ============================================================================
# Run in order. The chain stops at the first failure, because later entries
# usually depend on earlier ones (you cannot exercise a binary that failed to
# build, and the output would be noise).
#
# Edit them here rather than exporting env vars: the SubagentStop hook runs
# in a fresh shell that will not have your environment.

# Always run — in --hook mode (every implementer stop) and at review.
CHECKS=(
  "./scripts/check.sh"
  "go build -o envprobe ."
  "./scripts/exit-codes.sh"
)

# Run at review only, skipped in --hook mode. Move anything slow here once
# you know how long it takes; the hook fires often and latency compounds.
CHECKS_REVIEW_ONLY=(
)
# ============================================================================

set -uo pipefail

HOOK_MODE=0
[ "${1:-}" = "--hook" ] && HOOK_MODE=1

FAILED=0
mkdir -p .pipeline

# CHECK_CMD overrides the whole chain, for one-off manual runs.
if [ -n "${CHECK_CMD:-}" ]; then
  CHECKS=("$CHECK_CMD")
  CHECKS_REVIEW_ONLY=()
fi

# NOTE: macOS ships bash 3.2, where "${arr[@]}" on an EMPTY array is an
# unbound-variable error under `set -u`. Always length-check before expanding.
TO_RUN=()
if [ ${#CHECKS[@]} -gt 0 ]; then TO_RUN=("${CHECKS[@]}"); fi
if [ "$HOOK_MODE" -eq 0 ] && [ ${#CHECKS_REVIEW_ONLY[@]} -gt 0 ]; then
  TO_RUN+=("${CHECKS_REVIEW_ONLY[@]}")
fi

if [ ${#TO_RUN[@]} -eq 0 ]; then
  echo "ERROR: no checks configured. Edit the CHECKS array in scripts/guard.sh." >&2
  exit 2
fi

STEP=0
TOTAL=${#TO_RUN[@]}
for check in "${TO_RUN[@]}"; do
  STEP=$((STEP + 1))
  printf '=== [%d/%d] %s ===\n' "$STEP" "$TOTAL" "$check"

  # A configured check that isn't runnable is a setup error, not a code
  # failure. Say so plainly instead of blaming the diff.
  first="${check%% *}"
  if [ ! -x "$first" ] && ! command -v "$first" >/dev/null 2>&1; then
    printf '  ERROR: %s is not executable or not on PATH\n' "$first"
    printf '  Fix the CHECKS array in scripts/guard.sh, or chmod +x it.\n'
    exit 2
  fi

  if eval "$check"; then
    printf '  PASS\n\n'
  else
    printf '  FAIL: %s\n' "$check"
    if [ "$STEP" -lt "$TOTAL" ]; then
      printf '  Chain stopped. Remaining %d check(s) not run:\n' "$((TOTAL - STEP))"
      for skipped in "${TO_RUN[@]:$STEP}"; do printf '    - %s\n' "$skipped"; done
      printf '  Fix this one first, then re-run to reach the rest.\n'
    fi
    FAILED=1
    break
  fi
done

# --- 2. Agent-specific checks --------------------------------------------
# --- Collect changed content ---------------------------------------------
# CRITICAL: `git diff HEAD` does not see untracked files, and a new feature
# creates new files. Scanning only the diff means a brand-new file with a
# hardcoded secret or a panic("not implemented") passes every check below.
#
# So: added lines from tracked files, PLUS every line of each untracked file
# (all of it is new). Prefixed with '+' so one grep handles both.
# Deliberately does not use `git add -N`, which would mutate your index.

untracked_go() { git ls-files -o --exclude-standard -- '*.go' 2>/dev/null; }
untracked_all() { git ls-files -o --exclude-standard 2>/dev/null; }

# Untracked files get read whole, so skip anything that isn't small text.
# A stray binary (an un-gitignored ./bin/app) would otherwise make BSD grep
# report "Binary file matches" and turn the secret scan into a false failure.
# `grep -I` treats binary as non-matching on both GNU and BSD.
scannable() {
  [ -f "$1" ] || return 1
  LC_ALL=C grep -qI . "$1" 2>/dev/null || return 1
  # skip files over ~1MB; nothing hand-written is that big
  [ "$(wc -c < "$1" 2>/dev/null | tr -d ' ' || echo 0)" -lt 1048576 ] || return 1
  return 0
}

NEW_GO=$(
  { git diff -U0 HEAD -- '*.go' 2>/dev/null | grep -E '^\+' || true
    while IFS= read -r f; do
      scannable "$f" && sed 's/^/+/' "$f"
    done < <(untracked_go)
  } 2>/dev/null
)

NEW_ALL=$(
  { git diff -U0 HEAD 2>/dev/null | grep -E '^\+' || true
    while IFS= read -r f; do
      scannable "$f" && sed 's/^/+/' "$f"
    done < <(untracked_all)
  } 2>/dev/null
)

DIFF="$NEW_GO"   # kept for the erosion checks below

# --- Gate integrity -------------------------------------------------------
# The agent must not edit the things that judge it. permissions.deny blocks
# the Edit tool, but not `sed -i` from Bash, so check the diff too.
#
# NOTE: scripts/exit-codes.sh is deliberately NOT locked. It is a test suite,
# not a judge — it has to grow when a flag is added and shrink when one is
# removed. Growth is free; shrinkage is caught by scenario erosion below and
# checked against the plan at review.
#
# Honest limit: this file is checking itself. An agent that rewrites guard.sh
# defeats the check, since the hook re-reads the file each run. This catches
# the ordinary case; the deny-list is the real barrier, and your own review of
# the diff is the last one.
printf '\n=== gate integrity ===\n'
TAMPERED=$( { git diff --name-only HEAD 2>/dev/null; untracked_all; } \
  | grep -E '^(scripts/(guard|crap|check)\.sh|\.claude/)' || true)
if [ -n "$TAMPERED" ]; then
  printf '  FAIL: the diff modifies the pipeline gates themselves:\n'
  printf '%s\n' "$TAMPERED" | sed 's/^/    /'
  printf '  Gate files are off limits. If a check is genuinely wrong, say so\n'
  printf '  in your summary and let the human change it.\n'
  FAILED=1
else
  printf '  PASS\n'
fi

# --- Scenario erosion -----------------------------------------------------
# For test suites that are not *_test.go — CLI scenario scripts, golden
# files, fixtures. Adding scenarios is always fine. Removing them is fine
# too when the plan called for it (a flag was dropped), which is why this
# reports rather than forbids: the reviewer decides by checking the plan.
printf '\n=== scenario erosion ===\n'
SCEN_PATHS=("scripts/exit-codes.sh" "testdata" "test/scenarios")
EXISTING=()
for p in "${SCEN_PATHS[@]}"; do [ -e "$p" ] && EXISTING+=("$p"); done
if [ ${#EXISTING[@]} -gt 0 ]; then
  SDELTA=$(git diff --numstat HEAD -- "${EXISTING[@]}" 2>/dev/null \
    | awk '{a+=$1; d+=$2} END {print (a+0)-(d+0)}')
  if [ "${SDELTA:-0}" -lt 0 ]; then
    if [ "$HOOK_MODE" -eq 1 ]; then
      printf '  WARN: scenario/fixture code shrank by %s lines\n' "$((0 - SDELTA))"
      printf '  State this in your summary; the reviewer will fail it otherwise.\n'
    else
      printf '  FAIL: scenario/fixture code shrank by %s lines\n' "$((0 - SDELTA))"
      printf '  Requires explicit human approval.\n'
      FAILED=1
    fi
  else
    printf '  PASS\n'
  fi
else
  printf '  SKIP: no scenario paths present\n'
fi

printf '\n=== placeholder scan ===\n'
if [ -n "$NEW_GO" ]; then
  HITS=$(printf '%s\n' "$NEW_GO" | grep -E '^\+' | grep -Ein \
    'TODO|FIXME|panic\("(not implemented|unimplemented|TODO)|t\.Skip\(|t\.SkipNow|_ = err' \
    || true)
  if [ -n "$HITS" ]; then
    printf '  FAIL: placeholder or skipped test introduced:\n'
    printf '%s\n' "$HITS" | sed 's/^/    /'
    FAILED=1
  else
    printf '  PASS\n'
  fi
else
  printf '  SKIP: no Go changes\n'
fi

printf '\n=== test erosion ===\n'
if [ -n "$DIFF" ]; then
  DELTA=$(git diff --numstat HEAD -- '*_test.go' 2>/dev/null \
    | awk '{a+=$1; d+=$2} END {print (a+0)-(d+0)}')
  GONE=$(git diff -U0 HEAD -- '*_test.go' 2>/dev/null \
    | grep -E '^-' | grep -cE '^-func Test|^-func Fuzz' || true)
  if [ "${DELTA:-0}" -lt 0 ]; then
    if [ "$HOOK_MODE" -eq 1 ]; then
      # WARN, never fail, in hook mode. Failing here would block the stop,
      # leaving the agent unable to finish AND unable to explain itself — and
      # the only escape it can find is padding tests with filler until the
      # line count goes positive. Shrinking tests is a judgment call, so it
      # belongs at review, where it can reach a human.
      printf '  WARN: test code shrank by %s lines (%s deleted test funcs)\n' \
        "$((0 - DELTA))" "$GONE"
      printf '  Not blocking here. State this explicitly in your summary with\n'
      printf '  your justification — the reviewer will fail the diff otherwise.\n'
    else
      printf '  FAIL: test code shrank by %s lines (%s deleted test funcs)\n' \
        "$((0 - DELTA))" "$GONE"
      printf '  Requires explicit human approval. Do not accept silently.\n'
      FAILED=1
    fi
  else
    printf '  PASS: test code grew by %s lines\n' "${DELTA:-0}"
  fi
else
  printf '  SKIP\n'
fi

printf '\n=== secret scan ===\n'
if printf '%s\n' "$NEW_ALL" | grep -E '^\+' \
   | grep -qEi '(api[_-]?key|secret|password|token)["'"'"']?\s*[:=]\s*["'"'"'`][A-Za-z0-9/_+-]{16,}'; then
  printf '  FAIL: possible hardcoded secret in diff\n'
  FAILED=1
else
  printf '  PASS\n'
fi

printf '\n'
if [ "$FAILED" -ne 0 ]; then
  echo "GUARD FAILED - build/tests must be green and the diff clean before finishing"
  [ "$HOOK_MODE" -eq 1 ] && exit 2
  exit 1
fi
echo "GUARD PASSED"
exit 0
