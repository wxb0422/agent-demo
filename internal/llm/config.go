package llm

import (
	"net/http"
	"os"
)

type Config struct {
	BaseURL  string
	ApiKey   string
	Model    string
	MaxRetry int
	HttpCli  *http.Client
}

func GetDefaultConfig() *Config {
	return &Config{
		BaseURL:  envOrDefault("LLM_BASE_URL", "https://api.deepseek.com"),
		ApiKey:   envOrDefault("LLM_API_KEY", ""),
		Model:    envOrDefault("LLM_MODEL", "deepseek-flash"),
		MaxRetry: 5,
		HttpCli:  http.DefaultClient,
	}
}

func envOrDefault(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
