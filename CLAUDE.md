# CLAUDE.md

envprobe is a Go CLI that checks a machine has the tools and services a config
file names: binaries on PATH (with versions), TCP ports that answer, and the
Docker daemon.

## Commands

```bash
./scripts/check.sh         # gofmt, build, vet, test -race, lint -- run before committing
./scripts/exit-codes.sh    # exit-code contract; needs a built binary, not in CI
```

`check.sh` runs what CI runs. Reach for the individual `go` commands only to
iterate on one package.

`exit-codes.sh` drives the shipped binary in a temp `HOME`, so it is the only
thing covering `main`, `os.Exit`, `Execute` and the real config search path.

## Layout

- `main.go` -- `os.Exit(cmd.Execute())`. Nothing else.
- `cmd/` -- cobra tree, viper config loading, exit-code mapping. Everything
  that knows config *files* exist lives here.
- `internal/probe/` -- checks, results, rendering. Knows nothing about viper or
  the config search path; it takes `[]RawCheck` and gives back `[]Check`.

A check kind is a `checkFactory` registered in `registry` in
`internal/probe/config.go`. Adding one means: a `Run(ctx) Result` type, a
config struct decoded via `decodeWith` (`ErrorUnused` is on, so a typo'd key is
an error), a registry entry, and an `example.yaml` stanza.

## Exit codes

`exitCode` in `cmd/root.go` maps `checksFailedError` to 1 and everything else
to 2, so a new error type lands in "could not check" by default. Anything that
should exit 1 must reach that type. The table is in `doctor`'s `Long`.

## Conventions

- Design rationale lives in `docs/decisions.md`. Read the relevant section
  before changing existing behaviour, and record new decisions there rather
  than in code comments.
- Comments state what the code does, not why it was written that way. Drop the
  ones that restate the line below them.
- Errors are values. Return them; do not log-and-return, and do not call
  `os.Exit` outside `main`. Match with `errors.Is` / `errors.AsType`, never on
  message text. `errors.AsType` and `sync.WaitGroup.Go` are recent stdlib and
  deliberate -- do not rewrite them as `errors.As` or `wg.Add`/`go func`.
- Tests spawn no external processes. `go` is the exception. Anything else needs
  a seam.
- Do not add a branch, field or parameter that only a test can reach. If
  production has one call site, that is the signature.
- Tests using `t.Chdir` or `t.Setenv` must not call `t.Parallel` -- both mutate
  process-wide state.
- Diagnostics go to stderr. stdout carries results only, so `--json` stays
  parseable.
- Announce test-scope changes. Do not widen or narrow coverage inside a diff
  that is about something else.

## Commits

Plain ASCII only: no double quotes, backticks, `$` or `!`, so a message stays
safe in `git commit -m` from an interactive zsh. Conventional-commit prefixes
(`feat:`, `fix:`, `test:`, `chore:`, `ci:`, `refactor:`). Dependabot writes
`chore(deps):` and `ci(deps):` itself, set in `.github/dependabot.yml`.

Subjects are release notes. `.goreleaser.yaml` includes only `feat:`, `fix:`
and `chore(deps):`, so those are what users read when deciding whether to
upgrade and must say what changed for them, not how it was built. The rest
are read only by us.

Keep the subject short and imperative. A body is for what a subject cannot
carry -- a test-scope change, a contract that moved. Most commits do not need
one.
