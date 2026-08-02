# Design decisions

Why envprobe is built the way it is. Each entry is a choice that is easy to
undo by accident, with the reasoning that would be lost if someone did.

Read the relevant section before changing existing behaviour. Add an entry when
a decision is not visible in the diff that implements it.

------------------------------------------------------------------------

## Config schema

One shape for every check, no alternatives accepted:

```yaml
checks:
  - name: postgres      # display label, must be unique
    type: port          # key into the check registry
    with:               # payload, decoded per kind
      target: localhost:5432
```

`name` is the label shown in output and the key in `--json`. `target` is the
thing actually probed. They differ meaningfully for `port` -- an address makes
a poor label -- while for `binary` they usually match, so `target` defaults to
`name`.

`with:` is decoded by mapstructure with `ErrorUnused: true`, so a misspelled
key is a hard error naming the key rather than a silently ignored setting.

## The check registry is closed

Kinds come from a `map[string]checkFactory` in `internal/probe/config.go`. The
project ships the kinds; config only instantiates them. An unknown `type` is a
map miss, not a plugin lookup.

Adding a kind means four things: a type with a `Run(ctx) Result` method, a
config struct decoded via `decodeWith`, a registry entry, and a stanza in
`example.yaml`. `example_test.go` feeds the embedded template through the same
path the real command uses and asserts every registry kind appears, so adding a
kind without documenting it fails the build.

## LoadChecks builds, never runs

Constructing checks and running them are separate phases. `LoadChecks` collects
every problem via `errors.Join` rather than failing on the first, so one pass
reports everything wrong with a config file instead of one error per edit.

This is also what makes `config validate` possible: it is the build phase
without the run phase.

## Config loading lives in cmd, not probe

`internal/probe` takes `[]RawCheck` and returns `[]Check`. Viper, the search
path, and `ConfigFileUsed` all live in `cmd`. The split is why the sentinel
errors are exported from `probe` -- `cmd` matches them across the package
boundary -- while `cmd`'s own sentinels stay unexported.

## Config loading is explicit

There is no `cobra.OnInitialize`. It read config before *every* command and
exited on a parse error, so malformed YAML killed the one command meant to
diagnose it. Config is loaded by the commands that need it, and errors travel
to cobra as values.

For the same reason there are no package-level `var xCmd` and no `init()`.
`newRootCmd` / `newDoctorCmd` / `newConfigCmd` assemble the tree per
invocation, so tests get a fresh command with fresh flag values, and `doctor`
can be built with an injected loader.

## Three config subcommands, one job each

* `config example` prints the embedded template. No flags, no filesystem.
* `config init` writes it. `-o/--out` defaults to `envprobe.yaml`; the file is
  opened with `O_EXCL` so check-and-create is one syscall, and `--force` swaps
  to `O_TRUNC`.
* `config validate [file]` parses without running.

Split this way rather than overloading `config init -o -` for stdout: `init`
should mean "writes a file".

## Errors are values

`internal/probe/errors.go` holds the sentinels (`ErrNameRequired`,
`ErrDuplicateName`, `ErrTypeRequired`, `ErrUnknownType`, `ErrTargetRequired`)
and `CheckError{Index, Name, Err}`, shaped after `fs.PathError` -- context
fields plus a wrapped cause, with `Unwrap()` so the sentinel underneath stays
reachable. The sentinel answers *what* failed; `CheckError` answers *where*.

Tests match with `errors.Is` / `errors.AsType` wherever the error value is one
we own. Third-party decode errors keep substring matching, because identity
matching needs a value you can name.

### SilenceUsage is set in RunE, not on the root command

Cobra reads `SilenceUsage` *after* `cmd.execute()` returns, so setting it as
the first line of each `RunE` means flag- and `Args`-validation errors -- which
fail before `RunE` runs -- still print usage, while runtime failures do not.
Setting it on the root command silences both.

The earlier approach here was a `usageError` type plus `SetFlagErrorFunc`. It
was dropped because `SetFlagErrorFunc` never sees `Args` errors, so
`config example extra-arg` landed in the wrong bucket.

Note that cobra writes the usage block to `OutOrStderr()`, not to the `SetErr`
writer. In the real binary that is stderr; under a test that calls `SetOut` it
is the out buffer. Only the `Error:` line uses `SetErr`.

## envprobe never guesses what to check

There is no built-in default check set. `binary` is `exec.LookPath`, `port`
dials any `host:port`, `docker-daemon` runs `docker info` -- none of it knows
whether the machine is a laptop, a CI runner, or a UAT box, so no set of checks
is correct everywhere. A hardcoded set of developer tools is a wrong answer
delivered silently, and a green run against it means nothing.

So the two cases are kept separate:

* **No config file found** -- the user has not said what to check, so passing
  would be a lie. `doctor` reports it and exits 2.
* **A config file with an empty `checks:` list** -- an explicit choice, not a
  missing setup. Run nothing, exit 0.

`config init` and `config example` cover the "works out of the box" need, and
they teach the config, which is where the value is.

### Zero checks warn interactively and fail under `--ci`

The table renderer emits nothing for zero results, so without a message a run
that checked nothing looks like a crash. Plain `doctor` warns on stderr and
exits 0, preserving "an empty list is a valid choice" for interactive use, and
keeping `--json`'s stdout exactly `[]`.

Under `--ci` it is a failure instead. `--ci` is a gate, and a gate that passes
without verifying anything is broken; the realistic path to an empty list in CI
is a bad merge or a templating bug, not intent.

That failure returns *before* `RunAll`, unlike the "some checks failed" case
which renders first. There is nothing to run and nothing to show, and printing
`[]` before exiting non-zero would invite a consumer to read it as a parsed
empty result set rather than as "this never ran".

## Exit codes separate "found problems" from "could not check"

Exit 0 means everything checked passed, or there was nothing to check. Exit 1
means the checks ran and at least one failed. Exit 2 means envprobe could not
check at all -- no config file, no checks under `--ci`, a config it could not
build, a bad flag.

The distinction that matters is **"I checked and found problems"** versus
**"I couldn't check."** The first is a finding about the machine; the second is
nearly always a bug in the pipeline that invoked us. A gate fails on both, but
a human triages them differently, and without separate codes they are
indistinguishable without parsing stderr.

`grep` is the precedent -- 0 match, 1 no match, 2 error -- and it is the same
shape. Nagios plugins and `terraform plan -detailed-exitcode` order these the
other way, putting the error *above* the findings, but envprobe is a shell and
CI tool rather than a plugin polled by a monitoring daemon. Nothing goes near
126/127/128+N, which the shell owns.

`exitCode` in `cmd/root.go` maps `checksFailedError` to 1 and **defaults
everything else to 2**, so an error type added later lands in "could not check"
without anyone remembering to update the mapping. That is the safe direction to
fail. It matches with `errors.AsType` rather than a type switch, so a wrapper
upstream cannot silently defeat it.

`main` is `os.Exit(cmd.Execute())` and `Execute` returns an `int`. Keeping
`os.Exit` out of `cmd` is what makes the mapping testable.

------------------------------------------------------------------------

# Deferred

## `Result` is a junk drawer

`Path` and `Version` are meaningless for `portCheck`, which also throws away
the distinction between "connection refused" and "timed out" (both available
via `net.Error.Timeout()` / `errors.Is(err, context.DeadlineExceeded)`).

Left as-is deliberately: it is a flat struct at the output boundary, the
`jsonResult` DTO it converts to marks those fields `omitempty` so they do not
surface, and a uniform `Render` is worth more today than precise types. Revisit
when a kind needs a field the others cannot fake.

## `dockerDaemonCheck` has no tests

It shells out to `docker info` with no seam for faking `exec`. The sketch is a
`run func(ctx) error` field, nil meaning the real call. The constructor has to
leave it nil, because `cmp` treats two non-nil funcs as unequal and setting it
there would break `TestLoadChecks`.

Parked on the design question of whether a seam that exists only for tests
earns its keep. Tests otherwise spawn no external processes.

## No structured logging yet

`log/slog` was considered and declined for now. envprobe has one diagnostic
consumer -- a person, or a CI log a person reads -- and a level flag exists to
let different consumers ask for different volumes. There is no second consumer
to serve.

The concrete case was thinner than it looked. Per-check durations, the obvious
thing to log, are already in the results table (`render.go`). That left two
genuinely invisible things:

* **Which config file the search picked.** `doctor` never says, and
  `config validate` only prints a path when one was passed explicitly. Worth a
  plain `using <path>` line on stderr, not a logging framework.
* **The version command's error, swallowed in `binaryCheck.Run`.** A binary
  whose `--version` fails reports no version and no reason. That is a `Result`
  shape question -- see "`Result` is a junk drawer" -- and turning it into a
  log line would have been finding work for slog rather than fixing the hole.

Diagnostics stay plain `fmt.Fprint` to stderr, which keeps one channel and one
format. The trigger for revisiting is a second consumer: log shipping that
wants JSON, or enough diagnostic lines that people want to mute some and keep
others. One line per run is not that.

### The source line is a print, not error context

`doctor` prints `using <path>` before it checks the loader's error, so a config
that failed to build still names the file -- which is when a reader most needs
it. `configuredChecks` therefore returns the path alongside the build error;
it is empty only when no file was found, the one case with nothing to name.

`config validate` prints the same line on the same failure. It is the command
whose job is diagnosing config, so it was the worse one to leave silent: it
already names the file on success (`<path>: N checks OK`, on stdout, where the
path *is* the result) and now names it on failure too, on stderr, where it is a
diagnostic.

The alternative was to put the path in the error instead,
`fmt.Errorf("%s: %w", path, err)`, which is the shape the rest of the package
uses. It is wrong here. `LoadChecks` collects failures with `errors.Join`, so
its `Error()` is multi-line, and a prefix attaches to the first line only:

```
/abs/envprobe.yaml: checks[0] "alpha": unknown check type "nonsense"
checks[1] "beta": unknown check type "alsobad"
```

That reads as though the path belongs to `checks[0]`. A joined error cannot
take a prefix that applies to the whole set, so the file is named once, above
the error, by the code that is printing anyway.

Malformed YAML takes the prefix instead. It fails in `loadConfig`, which
returns a nil viper, so no path reaches the caller and the source line never
prints -- the one failure where a reader most needs the filename, since they
just edited it. The `errors.Join` objection above does not apply here: viper's
parse error is a single line, so `fmt.Errorf("%s: %w", v.ConfigFileUsed(), err)`
attaches the path to the whole error and not to one entry of a set.
`ConfigFileUsed()` is populated even though `ReadInConfig()` failed, so this
costs nothing and leaves the contract -- a non-nil error means there is nothing
to read -- intact.

`loadConfigFile` prefixes its own read error the same way, with the path it was
handed rather than `ConfigFileUsed()`. The user typed that path, so it is the
weaker case -- but a failure that names its file through one loader and not the
other is a difference with no reason behind it.

So the path reaches the user either way, by whichever route reads correctly:
above the error when the file parsed and its contents failed, inside the error
when the file did not parse at all.

## Platforms

CI runs on `ubuntu-latest` only. A matrix triples the minutes to test code with
no OS-specific paths; the trigger for adding one is shipping binaries for other
platforms, and what earns its keep there is `go test ./...` per OS rather than
executing the released artifacts -- path, env and `exec.LookPath` differences
are where the risk is.

Windows is out of scope. A tool that probes for `docker` and `java` on a dev
machine has a plausible audience that is entirely Unix. Revisit if someone
asks.

The config search path uses `os.UserHomeDir()` rather than a literal `$HOME`,
which expands to nothing wherever that variable is unset. It deliberately does
*not* use `os.UserConfigDir()`: that resolves to `~/Library/Application
Support` on macOS, which is not where a dev CLI is expected to look.
`~/.config/envprobe` is.
