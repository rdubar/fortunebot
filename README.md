# fortunebot

An AI-powered take on the classic Unix `fortune` command. Generates short, witty fortunes via the OpenAI API and delivers them instantly — every time — thanks to a stale-while-revalidate cache with background prefetching.

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
export OPENAI_API_KEY=sk-...
fortunebot
```

> Output: `🤖 "Your next best friend might just be a neural network — just don't ask it to borrow money!"`

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

If `OPENAI_API_KEY` isn't already in your environment, create a local env file:

```bash
mkdir -p ~/.config/fortunebot
cp examples/fortunebot.env.example ~/.config/fortunebot/fortunebot.env
$EDITOR ~/.config/fortunebot/fortunebot.env   # set OPENAI_API_KEY=sk-...
```

fortunebot picks this up automatically on startup. Keys in the env file take precedence over `config.json`.

## Usage

```
fortunebot [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--verbose` | off | Show config sources, cache state, and prefetch status |
| `--cache-ttl N` | 60 | Cache lifetime in seconds (0 = always fetch fresh) |
| `--no-cache` | false | Bypass cache entirely |
| `--clear-cache` | false | Delete cache before running |
| `--no-prefetch` | false | Disable background refresh |
| `--prompt TEXT` | built-in | Override the fortune prompt |
| `--model NAME` | gpt-4o-mini | OpenAI model to use |
| `--api-key KEY` | — | API key (prefer env var instead) |
| `--show-log` | — | Print full fortune history and exit |
| `-r`, `--log-random` | — | Print a random past fortune (no API call) |

**Examples:**

```bash
fortunebot                        # instant from cache; background refresh
fortunebot --verbose              # trace config sources and cache state
fortunebot --no-cache             # always call the API
fortunebot --cache-ttl 300        # cache for 5 minutes
fortunebot -r                     # random fortune from local log, no API
fortunebot --show-log             # view full fortune history
```

## How config is resolved

Settings are resolved in this order — first match wins:

1. CLI flags
2. Environment variables (`FORTUNEBOT_API_KEY` or `OPENAI_API_KEY`; `FORTUNEBOT_MODEL` or `OPENAI_MODEL`; `FORTUNEBOT_PROMPT`)
3. `~/.config/fortunebot/fortunebot.env` (or path in `FORTUNEBOT_ENV`)
4. `~/.config/fortunebot/config.json`
5. Built-in defaults (`gpt-4o-mini`, 60s TTL)

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

## Contributing

Issues and PRs welcome. The Go CLI in `cmd/fortunebot/` is the sole entry point — keep it that way.

## License

MIT — see [LICENSE](LICENSE).

## Author

**Roger Dubar** — [rdubar@gmail.com](mailto:rdubar@gmail.com) — [github.com/rdubar](https://github.com/rdubar)

Coding assistance: [Claude Code](https://claude.ai/code) (Anthropic) and OpenAI Codex.
