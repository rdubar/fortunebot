package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPrompt         = "Generate a very short, funny fortune cookie message about AI, programmers, or neural networks. Maximum 2 short sentences."
	defaultModelOpenAI    = "gpt-4o-mini"
	defaultModelAnthropic = "claude-haiku-4-5-20251001"
	cacheTTLDefault       = 60
)

// Paths
var (
	homeDir    = mustHome()
	configPath = filepath.Join(homeDir, ".config", "fortunebot", "config.json")
	dataDir    = filepath.Join(homeDir, ".local", "share", "fortunebot")
	cachePath  = filepath.Join(dataDir, "cache.json")
	logPath    = defaultLogPath()
)

type config struct {
	APIKey        string `json:"api_key"`
	AnthropicKey  string `json:"anthropic_api_key"`
	DefaultPrompt string `json:"default_prompt"`
	Model         string `json:"model"`
	Provider      string `json:"provider"`
}

type fortuneCache struct {
	Fortune   string  `json:"fortune"`
	Timestamp float64 `json:"timestamp"`
}

// mustHome returns the current user's home directory or panics if unavailable.
func mustHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return h
}

// defaultLogPath returns the path for the log file next to the executable.
func defaultLogPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "fortunebot.log")
	}
	return "fortunebot.log"
}

// loadDotEnv loads a fortunebot.env file from common locations.
func loadDotEnv() {
	paths := []string{}
	seen := map[string]bool{}
	add := func(p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	if custom := os.Getenv("FORTUNEBOT_ENV"); custom != "" {
		add(custom)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), "fortunebot.env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "fortunebot.env"))
	} else {
		add("fortunebot.env")
	}
	add(filepath.Join(filepath.Dir(configPath), "fortunebot.env"))
	add("fortunebot.env")

	for _, p := range paths {
		if err := applyEnvFile(p); err == nil {
			return
		}
	}
}

// applyEnvFile sets environment variables from a simple KEY=VAL file.
func applyEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
	return scanner.Err()
}

// loadConfig reads the optional JSON config file and fills defaults.
func loadConfig() config {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return config{DefaultPrompt: defaultPrompt}
	}
	var c config
	if err := json.Unmarshal(b, &c); err != nil {
		return config{DefaultPrompt: defaultPrompt}
	}
	if c.DefaultPrompt == "" {
		c.DefaultPrompt = defaultPrompt
	}
	// Model is intentionally not defaulted here; resolveProviderAndModel applies provider-specific defaults.
	return c
}

// resolveProviderAndModel resolves provider and model together since they are interdependent.
// Provider can be auto-detected from the model name prefix (claude-* → anthropic).
func resolveProviderAndModel(cliProvider, cliModel string, cfg config) (provider, model, providerSrc, modelSrc string) {
	// Resolve raw model value before applying defaults.
	rawModel, rawModelSrc := "", ""
	if strings.TrimSpace(cliModel) != "" {
		rawModel, rawModelSrc = cliModel, "--model flag"
	} else if v := os.Getenv("FORTUNEBOT_MODEL"); v != "" {
		rawModel, rawModelSrc = v, "env FORTUNEBOT_MODEL"
	} else if v := os.Getenv("OPENAI_MODEL"); v != "" {
		rawModel, rawModelSrc = v, "env OPENAI_MODEL"
	} else if strings.TrimSpace(cfg.Model) != "" {
		rawModel, rawModelSrc = cfg.Model, "config.json"
	}

	// Resolve provider: explicit sources first, then auto-detect from model name.
	switch {
	case strings.TrimSpace(cliProvider) != "":
		provider, providerSrc = cliProvider, "--provider flag"
	case os.Getenv("FORTUNEBOT_PROVIDER") != "":
		provider, providerSrc = os.Getenv("FORTUNEBOT_PROVIDER"), "env FORTUNEBOT_PROVIDER"
	case strings.TrimSpace(cfg.Provider) != "":
		provider, providerSrc = cfg.Provider, "config.json"
	case strings.HasPrefix(rawModel, "claude-"):
		provider, providerSrc = "anthropic", "auto-detected from model name"
	case rawModel != "":
		provider, providerSrc = "openai", "auto-detected from model name"
	default:
		provider, providerSrc = "openai", "default"
	}

	// Apply model with provider-aware defaults.
	if rawModel != "" {
		model, modelSrc = rawModel, rawModelSrc
	} else if provider == "anthropic" {
		model, modelSrc = defaultModelAnthropic, "built-in default"
	} else {
		model, modelSrc = defaultModelOpenAI, "built-in default"
	}
	return
}

// resolveAPIKeyWithSource resolves the API key for the active provider.
func resolveAPIKeyWithSource(cli, provider string, cfg config) (string, string) {
	if strings.TrimSpace(cli) != "" {
		return cli, "--api-key flag"
	}
	if provider == "anthropic" {
		if v := os.Getenv("FORTUNEBOT_ANTHROPIC_API_KEY"); v != "" {
			return v, "env FORTUNEBOT_ANTHROPIC_API_KEY"
		}
		if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
			return v, "env ANTHROPIC_API_KEY"
		}
		if strings.TrimSpace(cfg.AnthropicKey) != "" {
			return cfg.AnthropicKey, "config.json"
		}
	} else {
		if v := os.Getenv("FORTUNEBOT_API_KEY"); v != "" {
			return v, "env FORTUNEBOT_API_KEY"
		}
		if v := os.Getenv("OPENAI_API_KEY"); v != "" {
			return v, "env OPENAI_API_KEY"
		}
		if strings.TrimSpace(cfg.APIKey) != "" {
			return cfg.APIKey, "config.json"
		}
	}
	return "", "none found"
}

// resolvePromptWithSource resolves the prompt and explains where it came from.
func resolvePromptWithSource(cli string, cfg config) (string, string) {
	if strings.TrimSpace(cli) != "" {
		return cli, "--prompt flag"
	}
	if v := os.Getenv("FORTUNEBOT_PROMPT"); v != "" {
		return v, "env FORTUNEBOT_PROMPT"
	}
	if strings.TrimSpace(cfg.DefaultPrompt) != "" {
		return cfg.DefaultPrompt, "config.json"
	}
	return defaultPrompt, "built-in default"
}

// isErrorFortune detects cached error strings (we skip caching/logging them).
func isErrorFortune(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "[fortunebot]")
}

// loadCache reads the cached fortune, if present.
func loadCache() (*fortuneCache, error) {
	b, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var c fortuneCache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if isErrorFortune(c.Fortune) {
		return nil, fmt.Errorf("cache is error")
	}
	return &c, nil
}

// saveCache writes the fortune to disk unless it's an error message.
func saveCache(f string) {
	if isErrorFortune(f) {
		return
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Failed to create data dir: %v\n", err)
		return
	}
	payload, err := json.Marshal(fortuneCache{Fortune: f, Timestamp: float64(time.Now().Unix())})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Failed to marshal cache: %v\n", err)
		return
	}
	if err := os.WriteFile(cachePath, payload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Failed to write cache: %v\n", err)
	}
}

// cacheIsFresh reports whether the cache is younger than the TTL.
func cacheIsFresh(c *fortuneCache, ttl int) bool {
	age := time.Since(time.Unix(int64(c.Timestamp), 0))
	return age < time.Duration(ttl)*time.Second
}

// clearCache deletes the cache file if it exists.
func clearCache() {
	if err := os.Remove(cachePath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[fortunebot] Failed to clear cache: %v\n", err)
		} else {
			fmt.Println("[fortunebot] No cache to clear.")
		}
		return
	}
	fmt.Println("[fortunebot] Cache cleared.")
}

// logFortune appends a fortune to the log file (skipping error messages).
func logFortune(f string) {
	if isErrorFortune(f) {
		return
	}
	line := fmt.Sprintf("%d\t%s\n", time.Now().Unix(), f)
	fh, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Failed to write log: %v\n", err)
		return
	}
	defer fh.Close()
	if _, err := fh.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Failed to write log: %v\n", err)
	}
}

// printLog streams the log file to stdout.
func printLog() {
	b, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Println("[fortunebot] No log file found.")
		return
	}
	fmt.Print(string(b))
}

// generateFortune dispatches to the appropriate provider implementation.
func generateFortune(prompt, apiKey, model, provider string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("no API key provided (provider: %s)", provider)
	}
	switch provider {
	case "anthropic":
		return generateFortuneAnthropic(prompt, apiKey, model)
	default:
		return generateFortuneOpenAI(prompt, apiKey, model)
	}
}

func generateFortuneOpenAI(prompt, apiKey, model string) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    []msg{{Role: "system", Content: "You are a fortune cookie generator."}, {Role: "user", Content: prompt}},
		"max_tokens":  60,
		"temperature": 0.9,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return "🤖 " + strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func generateFortuneAnthropic(prompt, apiKey, model string) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 60,
		"system":     "You are a fortune cookie generator.",
		"messages":   []msg{{Role: "user", Content: prompt}},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Content) == 0 || strings.TrimSpace(parsed.Content[0].Text) == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return "🤖 " + strings.TrimSpace(parsed.Content[0].Text), nil
}

// startPrefetch spawns a detached worker process to refresh cache/log.
func startPrefetch(prompt, apiKey, model, provider string, verbose bool) {
	exe, err := os.Executable()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[fortunebot] Cannot start prefetch (no executable path): %v\n", err)
		}
		return
	}
	args := []string{"--prefetch-worker", "--prompt", prompt, "--model", model, "--provider", provider}
	if apiKey != "" {
		args = append(args, "--api-key", apiKey)
	}
	cmd := &os.ProcAttr{
		Dir:   filepath.Dir(exe),
		Env:   append(os.Environ(), fmt.Sprintf("FORTUNEBOT_VERBOSE=%t", verbose)),
		Files: []*os.File{os.Stdout, os.Stderr, os.Stderr},
	}
	proc, err := os.StartProcess(exe, append([]string{exe}, args...), cmd)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[fortunebot] Failed to start prefetch: %v\n", err)
		}
		return
	}
	defer proc.Release()
	if verbose {
		fmt.Printf("[fortunebot] Background prefetch started (pid %d).\n", proc.Pid)
	}
}

// runPrefetchWorker performs one fetch/save/log for the background process.
func runPrefetchWorker(prompt, apiKey, model, provider string) int {
	fortune, err := generateFortune(prompt, apiKey, model, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Background prefetch failed: %v\n", err)
		return 1
	}
	logFortune(fortune)
	saveCache(fortune)
	if os.Getenv("FORTUNEBOT_VERBOSE") == "true" || os.Getenv("FORTUNEBOT_VERBOSE") == "1" {
		fmt.Println("[fortunebot] Background prefetch complete; cache updated.")
	}
	return 0
}

func formatAge(ts float64) string {
	age := time.Since(time.Unix(int64(ts), 0))
	return fmt.Sprintf("%ds", int(age.Seconds()))
}

// maskKey partially masks an API key for display.
func maskKey(k string) string {
	if len(k) <= 8 {
		return k
	}
	return fmt.Sprintf("%s***%s", k[:4], k[len(k)-4:])
}

// randomFortuneFromLog picks a random fortune from the log file.
func randomFortuneFromLog() (string, error) {
	b, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log: %w", err)
	}
	lines := strings.Split(string(b), "\n")
	var fortunes []string
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			fortunes = append(fortunes, strings.TrimSpace(parts[1]))
		}
	}
	if len(fortunes) == 0 {
		return "", fmt.Errorf("no fortunes found in log")
	}
	return fortunes[rand.Intn(len(fortunes))], nil
}

func main() {
	loadDotEnv()
	cfg := loadConfig()

	var (
		flagPrompt       = flag.String("prompt", "", "Override the fortune prompt.")
		flagAPIKey       = flag.String("api-key", "", "API key for the active provider.")
		flagModel        = flag.String("model", "", "Model to use (e.g. gpt-4o-mini, claude-haiku-4-5-20251001).")
		flagProvider     = flag.String("provider", "", "API provider: openai or anthropic (auto-detected by default).")
		flagCacheTTL     = flag.Int("cache-ttl", cacheTTLDefault, "Cache TTL in seconds (0 disables cache).")
		flagNoCache      = flag.Bool("no-cache", false, "Disable cache.")
		flagClearCache   = flag.Bool("clear-cache", false, "Delete cache before running.")
		flagNoPrefetch   = flag.Bool("no-prefetch", false, "Disable background prefetch.")
		flagVerbose      = flag.Bool("verbose", false, "Verbose output.")
		flagQuiet        = flag.Bool("quiet", false, "Quiet output (default).")
		flagShowLog      = flag.Bool("show-log", false, "Print fortune log and exit.")
		flagPrefetchWork = flag.Bool("prefetch-worker", false, "Internal: run as prefetch worker.")
		flagLogRandom    = flag.Bool("log-random", false, "Print a random fortune from the log instead of calling the API.")
	)
	flag.BoolVar(flagLogRandom, "r", false, "Print a random fortune from the log instead of calling the API.")
	flag.Parse()

	verbose := *flagVerbose && !*flagQuiet

	if *flagPrefetchWork {
		prompt, _ := resolvePromptWithSource(*flagPrompt, cfg)
		provider, model, _, _ := resolveProviderAndModel(*flagProvider, *flagModel, cfg)
		apiKey, _ := resolveAPIKeyWithSource(*flagAPIKey, provider, cfg)
		os.Exit(runPrefetchWorker(prompt, apiKey, model, provider))
	}

	if *flagShowLog {
		printLog()
		return
	}

	if *flagLogRandom {
		f, err := randomFortuneFromLog()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fortunebot] %v\n", err)
			os.Exit(1)
		}
		if verbose {
			fmt.Println("[fortunebot] Showing random fortune from log (no API call).")
		}
		fmt.Println(f)
		return
	}

	if *flagClearCache {
		clearCache()
	}

	prompt, promptSrc := resolvePromptWithSource(*flagPrompt, cfg)
	provider, model, providerSrc, modelSrc := resolveProviderAndModel(*flagProvider, *flagModel, cfg)
	apiKey, apiSrc := resolveAPIKeyWithSource(*flagAPIKey, provider, cfg)

	if verbose {
		fmt.Printf("[fortunebot] Using provider: %s (source: %s)\n", provider, providerSrc)
		fmt.Printf("[fortunebot] Using model: %s (source: %s)\n", model, modelSrc)
		fmt.Printf("[fortunebot] Using prompt (source: %s)\n", promptSrc)
		if apiKey != "" {
			fmt.Printf("[fortunebot] Using API key from: %s (%s)\n", apiSrc, maskKey(apiKey))
		} else {
			fmt.Printf("[fortunebot] No API key found (sources checked: %s)\n", apiSrc)
		}
	}

	cacheTTL := *flagCacheTTL
	caching := !*flagNoCache && cacheTTL > 0

	if caching {
		if cache, err := loadCache(); err == nil && cache != nil {
			if cacheIsFresh(cache, cacheTTL) {
				if verbose {
					fmt.Printf("[fortunebot] Using fresh cache (age %s < TTL %ds).\n", formatAge(cache.Timestamp), cacheTTL)
				}
				fmt.Println(cache.Fortune)
				if !*flagNoPrefetch {
					startPrefetch(prompt, apiKey, model, provider, verbose)
				}
				return
			}
			if verbose {
				fmt.Printf("[fortunebot] Cache stale (age %s >= TTL %ds). Serving stale and refreshing now...\n", formatAge(cache.Timestamp), cacheTTL)
			}
			fmt.Println(cache.Fortune)
			if !*flagNoPrefetch {
				startPrefetch(prompt, apiKey, model, provider, verbose)
			}
			return
		}
		if verbose {
			fmt.Println("[fortunebot] No cache found. Fetching fresh fortune...")
		}
	} else {
		if verbose {
			fmt.Println("[fortunebot] Caching disabled. Fetching fresh fortune...")
		}
	}

	fortune, err := generateFortune(prompt, apiKey, model, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fortunebot] Error: %v\n", err)
		os.Exit(1)
	}
	logFortune(fortune)
	fmt.Println(fortune)

	if caching {
		saveCache(fortune)
		if !*flagNoPrefetch {
			startPrefetch(prompt, apiKey, model, provider, verbose)
		}
	}
}
