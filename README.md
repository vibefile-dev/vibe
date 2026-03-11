# vibe

<div align="center">
    <picture>
        <img alt="vibe" src="https://raw.githubusercontent.com/vibefile-dev/vibe/main/img/vibefile-logo.svg"/>
    </picture>
    <div>
    <a href="https://github.com/vibefile-dev/vibe/releases">
      <img src="https://img.shields.io/github/v/release/vibefile-dev/vibe?style=flat&label=Latest%20version" alt="Release">
    </a>
    <a href="https://github.com/vibefile-dev/vibe/actions/workflows/ci.yml">
      <img src="https://github.com/vibefile-dev/vibe/actions/workflows/ci.yml/badge.svg" alt="Build Status" height="20">
    </a>
     <a href="https://discord.gg/3TsMMvK8tV">
      <img src="https://img.shields.io/discord/1480004846324027578?style=flat&label=Join%20Discord" alt="Discord">
     </a>
     </div>
</div>

---

AI-powered task runner driven by plain-English recipes. An MVP implementation of the [Vibefile spec](https://github.com/vibefile-dev/spec).

## Install

```bash
go install github.com/vibefile-dev/vibe@latest
```

Or build from source:

```bash
git clone https://github.com/vibefile-dev/vibe.git
cd vibe
go build -o vibe .
```

## Quick start

Create a `Vibefile` in your project root or try `vibe init` to generate one for you:

```makefile
model = claude-sonnet-4-6
env = production

build:
    "compile and bundle the project for $(env)"

test:
    "run the test suite"

deploy: test build:
    "deploy to $(env) on fly.io"
```

Set your API key:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Run a target:

```bash
vibe run build
```

## Commands

| Command | Description |
|---------|-------------|
| `vibe run <target>` | Run a target and its dependencies |
| `vibe run <target> --dry` | Show generated script without executing |
| `vibe run <target> --recompile` | Force LLM regeneration for this target |
| `vibe run <target> --recompile-all` | Force LLM regeneration for target + all deps |
| `vibe list` | List all targets with descriptions |
| `vibe check` | Validate the Vibefile syntax |
| `vibe status` | Show compiled/uncompiled state of all targets |

## Compiled target caching

The first time a codegen target runs, the LLM-generated script is saved to `.vibe/compiled/<target>.sh` with a `.lock` file tracking the inputs. Subsequent runs skip the LLM entirely — no latency, no API cost.

The cache is invalidated automatically when any input changes:
- Recipe text edited in the Vibefile
- Variable values changed
- Relevant context files changed (e.g. `package.json`, `go.mod`)
- Model version changed

```
first run                          subsequent runs
──────────────────────────────     ───────────────────────────
vibe run build                     vibe run build
  → collect context                  → load .vibe/compiled/build.sh
  → call LLM API        (cost)      → run script              (free)
  → save .vibe/compiled/build.sh
  → run script
```

The `.vibe/compiled/` directory should be committed to version control so the team shares compiled targets.

## API key resolution

Keys are resolved in this order:

1. `--api-key` CLI flag
2. `VIBE_API_KEY` environment variable
3. Provider-specific env var (e.g. `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`)
4. `~/.vibeconfig` file

```yaml
# ~/.vibeconfig
default_model: claude-sonnet-4-6
anthropic_key: sk-ant-...
openai_key: sk-...
```
