package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DBType         string // "sqlite" or "postgres"
	DBDSN          string
	YouTubeAPIKeys []string
	CronSchedule   string
}

func LoadConfig() *Config {
	// Try loading .env if present
	loadDotEnv(".env")

	port := getEnv("PORT", "9090")
	dbType := strings.ToLower(getEnv("DB_TYPE", "sqlite"))
	dbDSN := getEnv("DB_DSN", "youtube_stats.db")

	keysStr := getEnv("YOUTUBE_API_KEYS", "")
	var apiKeys []string
	if keysStr != "" {
		parts := strings.Split(keysStr, ",")
		for _, k := range parts {
			trimmed := strings.TrimSpace(k)
			if trimmed != "" {
				apiKeys = append(apiKeys, trimmed)
			}
		}
	}

	cronSchedule := getEnv("CRON_SCHEDULE", "0 */6 * * *")

	return &Config{
		Port:           port,
		DBType:         dbType,
		DBDSN:          dbDSN,
		YouTubeAPIKeys: apiKeys,
		CronSchedule:   cronSchedule,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// remove quotes if wrapped
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}
