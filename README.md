# envprobe

[![CI](https://github.com/nchatzak/envprobe/actions/workflows/ci.yml/badge.svg)](https://github.com/nchatzak/envprobe/actions/workflows/ci.yml)

Check that a machine has the tools and services you expect — a laptop, a CI
runner, or a test box.

envprobe reads a config file that names what should be there, then verifies it:
executables on `PATH` (with their versions), TCP ports that answer, and the
Docker daemon. There are no built-in defaults. What you write in the config is
exactly what gets checked.

```console
$ envprobe doctor
using /Users/you/envprobe.yaml
✓  git            2.50.1   9ms
✓  Java           21.0.11  93ms
✓  Go             1.26.5   6ms
✗  terraform               0s
✓  postgres                2ms
✗  redis                   1ms
✓  docker-daemon           160ms
5 of 7 checks passed
```

## Install

Download a binary from the [latest
release](https://github.com/nchatzak/envprobe/releases/latest), or build from
source:

```bash
go install github.com/nchatzak/envprobe@latest
```

Linux and macOS. Windows is out of scope.

## Usage

Write a starter config, then run it:

```bash
envprobe config init     # writes ./envprobe.yaml
envprobe doctor          # run every check
```

`doctor` looks for `envprobe.yaml` in the current directory, then your home
directory, then `~/.config/envprobe`, and uses the first one it finds. The file
it chose is printed to stderr, so you always know which config produced the
output.

Checks run concurrently, each with a 5-second timeout, so one unreachable host
does not hold up the rest. A check that times out is reported as a failure.

### Commands

| Command | What it does |
| --- | --- |
| `envprobe doctor` | Run every configured check and print a table |
| `envprobe doctor --json` | Same, as JSON on stdout |
| `envprobe doctor --ci` | Exit non-zero when a check fails (see below) |
| `envprobe config init` | Write an example config to `./envprobe.yaml` (`-o` to change the path, `--force` to overwrite) |
| `envprobe config validate [file]` | Parse a config without running anything |
| `envprobe config example` | Print the annotated example config to stdout |
| `envprobe completion <shell>` | Print a shell completion script (`bash`, `zsh`, `fish`) |
| `envprobe --version` | Print the version (`-v` works too) |

### Shell completion

`envprobe completion <shell>` prints a script — it does not install one. Until
that script is where your shell looks, TAB does nothing and nothing says why.
On macOS with zsh:

```zsh
envprobe completion zsh > $(brew --prefix)/share/zsh/site-functions/_envprobe
```

Then **restart your shell**; the session that wrote the file will not pick it
up. `envprobe completion <shell> --help` has the paths and setup steps for each
shell — zsh, bash and fish, on both macOS and Linux — including the
`bash-completion` package that bash needs.

Two things that help does not mention:

- zsh ignores the file unless its completion system is on, silently. Check with
  `(( $+functions[compdef] )) && echo on || echo off`. If that prints `off`,
  add `autoload -Uz compinit && compinit` to `~/.zshrc` — but check first, as
  many tools run it for you when sourced. To confirm envprobe itself loaded:
  `print -l ${(k)_comps} | grep envprobe`.
- The script contains no command or flag names — it asks the binary on your
  `PATH` as you type, so upgrades bring their own and it never needs
  regenerating.

### JSON output

Results go to stdout, diagnostics to stderr, so `--json` stays pipeable:

```console
$ envprobe doctor --json | jq '.[] | select(.found | not) | .name'
"terraform"
"redis"
```

Each result carries `name`, `found` and `duration_ms`. `path` and `version` are
present only when the check found them, so a port check reports neither.

`problem` names the cause, and only when `found` alone would be ambiguous:
`connection refused`, `unreachable` and `timed out` are all a port that did not
answer. A binary that is not on `PATH` carries none, because `found: false`
says that. It can appear on a passing check — wrong `version_args` give
`found: true` with `problem: version command failed`.

In the table it follows the row in parentheses:

```console
$ envprobe doctor
✓  git    2.51.0  12ms
✗  redis          3ms  (connection refused)
```

The `5 of 7 checks passed` line is a diagnostic, so it goes to stderr in both
formats — stdout stays exactly the results array. It prints on every run that
had checks to run, whether or not `--ci` is set, so one grep finds the tally in
any log.

## Configuration

Every check has the same three fields:

- **`name`** — the label in the output and the key under `--json`. Must be unique.
- **`type`** — `binary`, `port`, or `docker-daemon`.
- **`with`** — the payload for that type. Keys are validated: an unknown key is
  an error that names the key, so a typo fails loudly instead of being ignored.

The three types:

| Type | Checks | `with:` |
| --- | --- | --- |
| `binary` | The executable is on `PATH`, and its version if you ask for one | `target` (defaults to `name`), `version_args` (optional) |
| `port` | Something is listening, which proves a service is running rather than merely installed | `target`, required, as `"host:port"` |
| `docker-daemon` | `docker info` answers, which the binary check alone cannot tell you | none |

For a worked config with every type and the pitfalls annotated — which tools
print their version to stderr, which want no dashes at all:

```bash
envprobe config example    # print it
envprobe config init       # write it to ./envprobe.yaml
```

An empty `checks:` list runs nothing and exits 0, with a warning on stderr.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Every check passed, or there was nothing to check |
| 1 | The checks ran and at least one failed (`--ci` only) |
| 2 | envprobe could not check: no config file, no checks under `--ci`, a config it could not build, or a bad flag |

Without `--ci`, a failing check is information, not an error: `doctor` reports
it and exits 0. `--ci` turns the same run into a gate. The 1/2 split matters
there — 1 says the environment is wrong, 2 says envprobe never got far enough to
form an opinion, and a pipeline should treat those differently.

```yaml
- name: Verify toolchain
  run: envprobe doctor --ci
```

## Development

```bash
./scripts/check.sh       # gofmt, build, vet, test -race, lint — what CI runs
./scripts/exit-codes.sh  # drives the built binary to verify the exit-code contract
```

Design rationale lives in [docs/decisions.md](docs/decisions.md).

## License

MIT. See [LICENSE](LICENSE).
