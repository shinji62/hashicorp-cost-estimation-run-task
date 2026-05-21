package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	ServerAddr string
	ServerPath string
	HMACKey    string

	// Logging configuration
	LogLevel  string
	LogFormat string

	// C3X configuration
	C3XBudgetLimit        float64
	C3XPricingAPIEndpoint string
	C3XAPIKey             string
	C3XAPIKeyFile         string
	C3XSaveDebugPlan      bool

	// GCP configuration
	GCPAPIKey string

	// AI Agent configuration
	GoogleAPIKey           string
	GeminiAPIKey           string
	GeminiModel            string
	AgentSecurityEnabled   bool
	AgentPricingEnabled    bool
	AgentComplianceEnabled bool
	AgentFailOnCritical    bool
}

// Load loads configuration from command line arguments, secret files, and environment variables
// Priority: CLI args > Secret files > Environment variables > Defaults
func Load() (*Config, error) {
	cfg := &Config{}

	// Define command-line flags
	addr := flag.String("addr", "", "the port the run task HTTP server will run on")
	path := flag.String("path", "", "the URL path for the run task to receive HTTP request from TFC or TFE")
	hmacKey := flag.String("hmac-key", "", "the customizable secret which TFC or TFE will use to sign requests to the run task")
	hmacKeyFile := flag.String("hmac-key-file", "", "path to file containing the HMAC key")

	c3xAPIKeyFile := flag.String("c3x-api-key-file", "", "path to file containing the C3X API key")
	gcpAPIKeyFile := flag.String("gcp-api-key-file", "", "path to file containing the GCP API key")

	logLevel := flag.String("log-level", "", "log level (DEBUG, INFO, WARN, ERROR)")
	logFormat := flag.String("log-format", "", "log format (text, json)")

	flag.Parse()

	// Load server configuration
	cfg.ServerAddr = loadString(*addr, os.Getenv("SERVER_ADDR"), "22180")
	cfg.ServerPath = loadString(*path, os.Getenv("SERVER_PATH"), "/runtask")

	// Load HMAC key with priority: CLI flag > file > env var
	cfg.HMACKey = loadSecret(*hmacKey, *hmacKeyFile, os.Getenv("HMAC_KEY"), "")

	// Load logging configuration
	cfg.LogLevel = loadString(*logLevel, os.Getenv("LOG_LEVEL"), "info")
	cfg.LogFormat = loadString(*logFormat, os.Getenv("LOG_FORMAT"), "text")

	// Load C3X configuration
	cfg.C3XBudgetLimit = loadFloat(os.Getenv("C3X_BUDGET_LIMIT"), 0)
	cfg.C3XPricingAPIEndpoint = loadString("", os.Getenv("C3X_PRICING_API_ENDPOINT"), "http://localhost:4000")

	// C3X API Key: CLI file flag > env file var > CLI/env direct value
	c3xAPIKeyFromFile := loadString("", os.Getenv("C3X_API_KEY_FILE"), "")
	cfg.C3XAPIKeyFile = loadString(*c3xAPIKeyFile, c3xAPIKeyFromFile, "")
	cfg.C3XAPIKey = loadSecret("", cfg.C3XAPIKeyFile, os.Getenv("C3X_API_KEY"), "")

	cfg.C3XSaveDebugPlan = loadBool(os.Getenv("C3X_SAVE_DEBUG_PLAN"), false)

	// Load GCP configuration
	cfg.GCPAPIKey = loadSecret("", *gcpAPIKeyFile, os.Getenv("GCP_API_KEY"), "")

	// Load AI Agent configuration
	// Note: The ADK library requires GOOGLE_API_KEY or GEMINI_API_KEY to be set as environment variables
	// It does not support passing API keys directly, so we only read from environment variables
	cfg.GoogleAPIKey = loadString("", os.Getenv("GOOGLE_API_KEY"), "")
	cfg.GeminiAPIKey = loadString("", os.Getenv("GEMINI_API_KEY"), "")
	cfg.GeminiModel = loadString("", os.Getenv("GEMINI_MODEL"), "gemini-3.1-flash-lite")
	cfg.AgentSecurityEnabled = loadBool(os.Getenv("AGENT_SECURITY_ENABLED"), false)
	cfg.AgentPricingEnabled = loadBool(os.Getenv("AGENT_PRICING_ENABLED"), false)
	cfg.AgentComplianceEnabled = loadBool(os.Getenv("AGENT_COMPLIANCE_ENABLED"), false)
	cfg.AgentFailOnCritical = loadBool(os.Getenv("AGENT_FAIL_ON_CRITICAL"), false)

	return cfg, nil
}

// loadString returns the first non-empty string value
func loadString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// loadSecret loads a secret with priority: direct value > file > fallback
func loadSecret(directValue, filePath, fallbackValue, defaultValue string) string {
	// Priority 1: Direct value (from CLI flag or explicit setting)
	if directValue != "" {
		return directValue
	}

	// Priority 2: File path
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			// Log warning but continue - don't fail on missing secret files
			fmt.Fprintf(os.Stderr, "Warning: failed to read secret file %s: %v\n", filePath, err)
		} else {
			return strings.TrimSpace(string(content))
		}
	}

	// Priority 3: Fallback value (usually from env var)
	if fallbackValue != "" {
		return fallbackValue
	}

	// Priority 4: Default value
	return defaultValue
}

// loadFloat parses a float from string, returns default on error
func loadFloat(value string, defaultValue float64) float64 {
	if value == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// loadBool parses a boolean from string, returns default on error
func loadBool(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

// GetAPIKey returns the appropriate API key for Gemini/Google AI
func (c *Config) GetAPIKey() string {
	if c.GoogleAPIKey != "" {
		return c.GoogleAPIKey
	}
	return c.GeminiAPIKey
}

// HasAIEnabled returns true if AI agents can be used
func (c *Config) HasAIEnabled() bool {
	return c.GetAPIKey() != ""
}
