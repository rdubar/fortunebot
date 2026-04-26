# fortunebot

An AI-powered take on the classic Unix `fortune` command. Generates short, witty fortunes via the OpenAI or Anthropic API and delivers them instantly — every time — thanks to a stale-while-revalidate cache with background prefetching.

Built in Go with zero external dependencies.

## Features

- **Instant responses** — serves from cache while silently refreshing in the background
- **Stale-while-revalidate** — never blocks on an API call; stale cache is served immediately and replaced for next time
- **Configurable prompt** — change the fortune style via flag, environment variable, or config file
- **Multi-source config resolution** — flags → environment → config file → built-in defaults, with `--verbose` tracing each source
- **Fortune log** — every new fortune is appended to a local log; replay a random past fortune offline with `-r`
- **XDG-compliant paths** — config in `~/.config/fortunebot/`, data in `~/.local/share/fortunebot/`
- **Zero dependencies** — stdlib only; single static binary

## Quick start

```bash
go install github.com/rdubar/fortunebot/cmd/fortunebot@latest
export OPENAI_API_KEY=sk-...      # or ANTHROPIC_API_KEY=sk-ant-...
fortunebot
```

> `🤖 "Your next best friend might just be a neural network — just don't ask it to borrow money!"`

## Installation

**Option A — `go install` (no clone needed):**
```bash
go install github.com/rdubar/fortunebot/cmd/fortunebot@latest
```
Installs to `$(go env GOPATH)/bin` — ensure that's on your `PATH`.

**Option B — clone and build:**
```bash
git clone https://github.com/rdubar/fortunebot.git
cd fortunebot
make install          # builds and installs to ~/.local/bin
```
Ensure `~/.local/bin` is on your PATH:
```bash
export PATH="$HOME/.local/bin:$PATH"   # add to ~/.zshrc
```

**To update:** re-run either install command. `make install` rebuilds from source automatically.

## API key setup

If your API key isn't already exported, create a local env file:

```bash
mkdir -p ~/.config/fortunebot
cp examples/fortunebot.env.example ~/.config/fortunebot/fortunebot.env
$EDITOR ~/.config/fortunebot/fortunebot.env
```

Set `OPENAI_API_KEY` for OpenAI, or `ANTHROPIC_API_KEY` for Claude. fortunebot picks this up automatically on startup.

## Usage

```
fortunebot [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--provider NAME` | auto | API provider: `openai` or `anthropic` (auto-detected from model name) |
| `--model NAME` | provider default | Model to use (e.g. `gpt-4o-mini`, `claude-sonnet-4-6`) |
| `--api-key KEY` | — | API key for the active provider (prefer env var instead) |
| `--prompt TEXT` | built-in | Override the fortune prompt |
| `--verbose` | off | Show provider, model, config sources, cache state, and prefetch status |
| `--cache-ttl N` | 60 | Cache lifetime in seconds (0 = always fetch fresh) |
| `--no-cache` | false | Bypass cache entirely |
| `--clear-cache` | false | Delete cache before running |
| `--no-prefetch` | false | Disable background refresh |
| `--update` | — | Update fortunebot to the latest version via `go install` |
| `--status` | — | Show resolved config, API key, paths, and cache state, then exit |
| `--show-log` | — | Print full fortune history and exit |
| `-r`, `--log-random` | — | Print a random past fortune (no API call) |

**Examples:**

```bash
fortunebot                                         # instant from cache; background refresh
fortunebot --verbose                               # trace provider, model, cache state
fortunebot --provider anthropic                    # use Claude (reads ANTHROPIC_API_KEY)
fortunebot --model claude-sonnet-4-6               # auto-selects Anthropic provider
fortunebot --no-cache                              # always call the API
fortunebot --cache-ttl 300                         # cache for 5 minutes
fortunebot -r                                      # random fortune from local log, no API
fortunebot --show-log                              # view full fortune history
```

## How config is resolved

Settings are resolved in this order — first match wins:

1. CLI flags (`--provider`, `--model`, `--api-key`, `--prompt`, …)
2. Environment variables
3. `~/.config/fortunebot/fortunebot.env` (or path in `FORTUNEBOT_ENV`)
4. `~/.config/fortunebot/config.json`
5. Built-in defaults (60s TTL; model default depends on provider)

**Provider auto-detection:** if no provider is set explicitly, fortunebot infers it from the model name — `claude-*` models use Anthropic, everything else uses OpenAI.

**Environment variables by provider:**

| Variable | Provider | Purpose |
|----------|----------|---------|
| `OPENAI_API_KEY` | OpenAI | API key |
| `ANTHROPIC_API_KEY` | Anthropic | API key |
| `FORTUNEBOT_API_KEY` | OpenAI | API key (fortunebot-specific) |
| `FORTUNEBOT_ANTHROPIC_API_KEY` | Anthropic | API key (fortunebot-specific) |
| `FORTUNEBOT_PROVIDER` | — | Force provider (`openai` or `anthropic`) |
| `FORTUNEBOT_MODEL` | — | Model name |
| `FORTUNEBOT_PROMPT` | — | Fortune prompt |

Run with `--verbose` to see exactly which source each value came from.

## How background prefetch works

On each run, fortunebot:

1. Checks the cache. If fresh (within TTL), serves immediately.
2. Spawns a detached subprocess to fetch the next fortune from the API.
3. The subprocess saves to `~/.local/share/fortunebot/cache.json` and appends to `fortunebot.log`.
4. The main process exits — no waiting.

If the cache is stale, the stale fortune is served immediately while the subprocess replaces it. The result is sub-millisecond response time on every run after the first.

## Development

```bash
make build    # compile to ./fortunebot
make run      # build and run
make install  # install to ~/.local/bin
make clean    # remove local binary
make uninstall
```

Or without make:
```bash
go run ./cmd/fortunebot --verbose
go build -trimpath -ldflags="-s -w" -o fortunebot ./cmd/fortunebot
```

Requires Go 1.21+. No external dependencies.

## Background: the Unix fortune tradition

The [`fortune`](https://en.wikipedia.org/wiki/Fortune_(Unix)) program has been a staple of Unix systems since the 1970s — a simple utility that prints a random quote, joke, or aphorism from a flat text database. For decades it was the standard way to greet terminal users with something unexpected.

fortunebot brings that tradition into the AI age: instead of drawing from a static file, it generates a fresh fortune on demand using a large language model, with a stale-while-revalidate cache so it stays instant.

## Contributing

Issues and PRs welcome. The Go CLI in `cmd/fortunebot/` is the sole entry point — keep it that way.

## License

MIT — see [LICENSE](LICENSE).

## Author

**Roger Dubar** — [rdubar@gmail.com](mailto:rdubar@gmail.com) — [github.com/rdubar](https://github.com/rdubar)

Coding assistance: [Claude Code](https://claude.ai/code) (Anthropic) and [OpenAI Codex](https://openai.com/codex).
