# fortunebot (Go)

AI-powered take on the classic Unix `fortune`: short, funny fortunes generated on the fly via the OpenAI Responses API.

## Setup
Requires Go 1.21+.

Quick start (uses your existing `OPENAI_API_KEY`/`FORTUNEBOT_API_KEY` if set):
```bash
gh repo clone rdubar/fortunebot
cd fortunebot
make install   # builds and copies to ~/.local/bin/fortunebot
fortunebot     # prints a fortune
```

If you don’t export a key already, create an env file next to the binary:
```bash
cp examples/fortunebot.env.example fortunebot.env
nano fortunebot.env   # set OPENAI_API_KEY=sk-...
```
Then re-run `fortunebot`.

Tip: each run prefetches the next fortune in the background, so subsequent runs are instant. `fortunebot -r` shows a random fortune from the log without calling the API.

## Optional extras
- Optional prompt override: set `FORTUNEBOT_PROMPT` in the env file (or export it) to change the fortune style without CLI flags.
- Copy `examples/config.example.json` → `config.json` for non-secret defaults (prompt/model). Keep secrets in the env file.
- Build: `go build -o fortunebot ./cmd/fortunebot` or `make build`
- Install to `~/.local/bin` (no sudo): `make install` (ensure `~/.local/bin` is on your PATH)
- Run without building: `go run ./cmd/fortunebot`

## How config is resolved
1) CLI flags (`--api-key`, `--prompt`, `--cache-ttl`, etc.)
2) Environment variables / `fortunebot.env`: `FORTUNEBOT_API_KEY` or `OPENAI_API_KEY`; `FORTUNEBOT_MODEL` or `OPENAI_MODEL`.
3) `~/.config/fortunebot/config.json` (or local `config.json` if you choose).
4) Built-in defaults (`gpt-4o-mini`, short fortune prompt).

Recommendation: put API keys in `fortunebot.env` (gitignored) or real env vars. Use `config.json` only for non-secret defaults.

## Usage
- Default: `./fortunebot` — instant return if cache is fresh; background prefetch for next run (60s TTL).
- Always fresh: `./fortunebot --cache-ttl 0` or `--no-cache`
- Adjust cache TTL: `./fortunebot --cache-ttl 120`
- Clear cache: `./fortunebot --clear-cache`
- Verbosity: quiet by default; enable chatter with `./fortunebot --verbose` (or re-quiet with `--quiet`)
- Prefetch control: disable with `./fortunebot --no-prefetch`
- View log: `./fortunebot --show-log`
- Random from log (no API): `./fortunebot --log-random` or `./fortunebot -r`

### Background prefetch
- When a cache is fresh, the current fortune is printed immediately and a background subprocess fetches the next one. In quiet mode, only errors show; in `--verbose`, you’ll see “Background prefetch started/complete.”
- When a cache is stale, the stale fortune is printed immediately, and a background fetch replaces it for the next run.
- The background worker writes to `cache.json` (in `~/.local/share/fortunebot/`) and appends to `fortunebot.log` (next to the binary, gitignored). The main command never blocks on these background fetches.

## Contributing
- Issues and PRs welcome. Keep the Go CLI as the primary entry point.

## License
MIT

## Author
Roger Dubar — rdubar@gmail.com — GitHub: [rdubar](https://github.com/rdubar)

Coding assistance: OpenAI Codex.
