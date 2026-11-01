package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type config struct{ Host, Port, Provider, APIKey, Model, Password string }

const defaultOpenRouterModel = "google/gemini-3.8-flash"

// Read a small dotenv format without changing process-wide environment variables.
// Support KEY=value, export, whole-line comments and single/double quoted values.
// Existing environment values (including empty ones) always win.
func loadConfig(filename string, lookup func(string) (string, bool)) (config, error) {
	values := make(map[string]string)
	if filename != "" {
		file, err := os.Open(filename)
		if err != nil && (!os.IsNotExist(err) || filename != ".env") {
			return config{}, fmt.Errorf("open configuration: %w", err)
		}
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 4096), 64<<10)
			for line := 1; scanner.Scan(); line++ {
				text := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
				if text == "" || strings.HasPrefix(text, "#") {
					continue
				}
				text = strings.TrimPrefix(text, "export ")
				key, value, ok := strings.Cut(text, "=")
				key, value = strings.TrimSpace(key), strings.TrimSpace(value)
				if !ok || key == "" || strings.ContainsAny(key, " \t") {
					return config{}, fmt.Errorf("invalid configuration at line %d", line)
				}
				if strings.HasPrefix(value, "\"") {
					decoded, err := strconv.Unquote(value)
					if err != nil {
						return config{}, fmt.Errorf("invalid quoted value at line %d", line)
					}
					value = decoded
				} else if strings.HasPrefix(value, "'") {
					if len(value) < 2 || !strings.HasSuffix(value, "'") {
						return config{}, fmt.Errorf("invalid quoted value at line %d", line)
					}
					value = value[1 : len(value)-1]
				}
				values[key] = value
			}
			if err := scanner.Err(); err != nil {
				return config{}, fmt.Errorf("read configuration: %w", err)
			}
		}
	}
	get := func(key, fallback string) string {
		if value, ok := lookup(key); ok {
			return value
		}
		if value, ok := values[key]; ok {
			return value
		}
		return fallback
	}
	provider := strings.ToLower(strings.TrimSpace(get("AI_PROVIDER", "openai")))
	if provider == "" {
		provider = "openai"
	}
	if provider != "openai" && provider != "openrouter" {
		return config{}, fmt.Errorf("AI_PROVIDER must be openai or openrouter")
	}
	keyName, modelName, defaultModel := "OPENAI_API_KEY", "OPENAI_MODEL", "gpt-4.1"
	if provider == "openrouter" {
		keyName, modelName, defaultModel = "OPENROUTER_API_KEY", "OPENROUTER_MODEL", defaultOpenRouterModel
	}
	cfg := config{Host: get("HOST", "0.0.0.0"), Port: get("PORT", "3000"), Provider: provider, APIKey: strings.TrimSpace(get(keyName, "")), Model: strings.TrimSpace(get(modelName, defaultModel)), Password: get("LOOKUP_PASSWORD", "")}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = defaultModel
	}
	return cfg, nil
}
