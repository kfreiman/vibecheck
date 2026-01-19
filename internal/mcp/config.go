package mcp

import (
	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds the configuration for the MCP server
type Config struct {
	StoragePath string `env:"STORAGE_PATH" env-default:"./storage" env-description:"Storage directory path"`
	StorageTTL  string `env:"STORAGE_TTL" env-default:"24h" env-description:"Default TTL for document cleanup (e.g., 24h, 1h30m)"`
	Port        int    `env:"PORT" env-default:"8080" env-description:"HTTP server port"`
	LogDebug    bool   `env:"DEBUG" env-default:"false" env-description:"Enable debug logging"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// WithStoragePath sets the storage path
func (c Config) WithStoragePath(path string) Config {
	c.StoragePath = path
	return c
}

// WithStorageTTL sets the storage TTL
func (c Config) WithStorageTTL(ttl string) Config {
	c.StorageTTL = ttl
	return c
}

// WithPort sets the server port
func (c Config) WithPort(port int) Config {
	c.Port = port
	return c
}

// WithLogDebug enables or disables debug logging
func (c Config) WithLogDebug(debug bool) Config {
	c.LogDebug = debug
	return c
}
