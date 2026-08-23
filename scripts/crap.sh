#!/usr/bin/env bash
# CRAP gate. crap4go is the tool. No fallbacks.
#
#   CRAP(fn) = CC² x (1 - coverage)³ + CC
#
# Runs at REVIEW time, not on every implementer stop — crap4go re-runs the
# whole suite, so it's too slow for the fast gate.
#
# Two modes:
#   ./scripts/crap.sh baseline   snapshot current scores, always exit 0
#   ./scripts/crap.sh            score and gate against that snapshot
#
# The baseline is what makes a tight threshold usable on a real codebase.
# Functions the diff made worse are REGRESSIONS and fail. Functions that were
# already bad are PRE-EXISTING: reported for the simplifier, not blocking.
#
# Env:
#   CRAP_THRESHOLD   default 15   fail above this
#   CRAP_WARN        default 10   report between warn and threshold
#   CRAP_TEST_CMD    passed to --test-command
#   CRAP_WORKERS     passed to --max-workers
#   CRAP_FILTER      path fragments to scope the report

set -uo pipefail

THRESHOLD="${CRAP_THRESHOLD:-15}"
WARN="${CRAP_WARN:-10}"
PIPE_DIR=".pipeline"
REPORT="$PIPE_DIR/crap.txt"
BASELINE="$PIPE_DIR/crap-baseline.txt"
mkdir -p "$PIPE_DIR"

MODE="gate"
[ "${1:-}" = "baseline" ] && { MODE="baseline"; shift; }

command -v go >/dev/null 2>&1 || { echo "ERROR: go toolchain not found" >&2; exit 2; }

GOBIN="$(go env GOBIN)"; [ -n "$GOBIN" ] || GOBIN="$(go env GOPATH)/bin"
export PATH="$GOBIN:$PATH"

if ! command -v crap4go >/dev/null 2>&1; then
  echo "  crap4go not found; installing..."
  go install github.com/unclebob/crap4go/cmd/crap4go@latest || {
    echo "ERROR: failed to install crap4go" >&2; exit 2; }
  command -v crap4go >/dev/null 2>&1 || {
    echo "ERROR: crap4go not on PATH. Add $GOBIN to PATH." >&2; exit 2; }
fi

ARGS=()
[ -n "${CRAP_TEST_CMD:-}" ] && ARGS+=(--test-command "$CRAP_TEST_CMD")
[ -n "${CRAP_WORKERS:-}" ]  && ARGS+=(--max-workers "$CRAP_WORKERS")
if [ "$#" -gt 0 ]; then ARGS+=("$@")
elif [ -n "${CRAP_FILTER:-}" ]; then
  # shellcheck disable=SC2206
  ARGS+=(${CRAP_FILTER})
fi

OUT="$REPORT"
[ "$MODE" = "baseline" ] && OUT="$BASELINE"

# macOS ships bash 3.2, where "${arr[@]}" on an EMPTY array is an
# unbound-variable error under `set -u`. ARGS is empty in the common case
# (no test command, no workers, no filter), so this must be branched.
if [ ${#ARGS[@]} -gt 0 ]; then
  crap4go "${ARGS[@]}" > "$OUT" 2>&1 || {
    cat "$OUT" >&2; echo "ERROR: crap4go failed" >&2; exit 2; }
else
  crap4go > "$OUT" 2>&1 || {
    cat "$OUT" >&2; echo "ERROR: crap4go failed" >&2; exit 2; }
fi

if [ "$MODE" = "baseline" ]; then
  COUNT=$(awk '$NF ~ /^[0-9]+(\.[0-9]+)?$/' "$BASELINE" | wc -l | tr -d ' ')
  echo "  baseline snapshot written: $COUNT scored functions"
  exit 0
fi

cat "$REPORT"

# Report rows: Function Package CC Cov% CRAP  (neither name contains spaces)
[ -f "$BASELINE" ] || : > "$BASELINE"

awk -v t="$THRESHOLD" -v w="$WARN" '
  function isnum(x) { return x ~ /^[0-9]+(\.[0-9]+)?$/ }

  NR == FNR {                                   # baseline
    if (isnum($NF)) base[$2 "." $1] = $NF + 0
    next
  }
  {
    if ($NF == "N/A" || $NF == "n/a") { na++; next }
    if (!isnum($NF)) next
    k = $2 "." $1
    c = $NF + 0
    n++; total += c
    known = (k in base)
    prev = known ? base[k] : 0

    if (c > t) {
      if (!known || c > prev + 0.05) {
        reg++
        regs = regs sprintf("    %-28s %-18s %6.1f%s\n", $1, $2,
                 c, known ? sprintf("  (was %.1f)", prev) : "  (new)")
      } else {
        pre++
        pres = pres sprintf("    %-28s %-18s %6.1f\n", $1, $2, c)
      }
    } else if (c > w) {
      warn++
    }
  }
  END {
    if (n == 0 && na == 0) { print "\n  SKIP: no scored functions"; exit 0 }
    printf "\n  scored: %d   average CRAP: %.1f   warn: >%s   fail: >%s\n",
           n, (n ? total/n : 0), w, t
    if (na > 0)   printf "  %d N/A - unmeasured, not clean\n", na
    if (warn > 0) printf "  %d in warn band (%s-%s)\n", warn, w, t

    if (pre > 0) {
      printf "\n  PRE-EXISTING over threshold (not blocking; simplifier worklist):\n"
      printf "%s", pres
    }
    if (reg > 0) {
      printf "\n  REGRESSIONS - this diff made these worse or added them:\n"
      printf "%s", regs
      printf "\n  These are the implementers responsibility: write it simpler,\n"
      printf "  or add the tests the plan called for. Do not refactor\n"
      printf "  pre-existing code to compensate.\n"
      exit 1
    }
    print "\n  no CRAP regressions in this diff"
    exit 0
  }
' "$BASELINE" "$REPORT"
