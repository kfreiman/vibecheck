package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/kfreiman/vibecheck/internal/mcp"
	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog"
	"github.com/spf13/cobra"
)

// cmdConfig holds all configuration for the command line
type cmdConfig struct {
	Format string `env:"LOG_FORMAT" env-default:"text" env-description:"Log output format (text or json)"`
	Level  string `env:"LOG_LEVEL" env-default:"info" env-description:"Log level (debug, info, warn, error)"`
}

// createLogger creates a slog logger from the configuration
func createLogger(conf cmdConfig) *slog.Logger {
	// Parse log level
	var level slog.Level
	switch conf.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create zerolog logger
	var zerologLogger zerolog.Logger
	if conf.Format == "json" {
		zerologLogger = zerolog.New(os.Stderr)
	} else {
		zerologLogger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	}

	// Create slog handler
	loggerConfig := slogzerolog.Option{
		Level:  level,
		Logger: &zerologLogger,
	}.NewZerologHandler()

	logger := slog.New(loggerConfig)

	// Set as default logger
	log.SetFlags(0)
	slog.SetDefault(logger)

	return logger
}

// mcpServerCmd represents the mcp-server command
var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Start an MCP server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Load command configuration from environment variables
		var cmdConf cmdConfig
		if err := cleanenv.ReadEnv(&cmdConf); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load command config: %v\n", err)
			os.Exit(1)
		}

		// Create logger
		logger := createLogger(cmdConf)

		// Load MCP configuration from environment variables
		cfg, err := mcp.LoadConfig()
		if err != nil {
			logger.ErrorContext(ctx, "failed to load MCP config",
				"error", err,
			)
			os.Exit(1)
		}

		// Parse storage TTL for logging
		ttl := parseDuration(cfg.StorageTTL)

		logger.InfoContext(ctx, "mcp server starting",
			"port", cfg.Port,
			"storage_path", cfg.StoragePath,
			"storage_ttl", ttl,
			"endpoints", []string{"/mcp", "/health/live", "/health/ready"},
		)

		// Create the MCP server
		srv, err := mcp.NewServer(cfg, logger)
		if err != nil {
			logger.ErrorContext(ctx, "failed to create MCP server",
				"error", err,
			)
			os.Exit(1)
		}

		// Start the server
		if err := srv.ListenAndServe(); err != nil {
			logger.ErrorContext(ctx, "failed to start MCP server",
				"error", err,
			)
			os.Exit(1)
		}
	},
}

// parseDuration parses a duration string and returns a duration value
func parseDuration(d string) string {
	return d
}

// cvCmd represents the cv command
var cvCmd = &cobra.Command{
	Use:   "cv",
	Short: "Display example CV data in markdown format",
	Long: `Display example CV/resume data in markdown format.

This command displays CV data in markdown format via the MCP server.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		// Create a default logger for cv command
		var cmdConf cmdConfig
		if err := cleanenv.ReadEnv(&cmdConf); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load command config: %v\n", err)
			os.Exit(1)
		}
		logger := createLogger(cmdConf)
		logger.InfoContext(ctx, "cv command executed - MCP server required",
			"instruction", "run: vibecheck mcp-server",
		)
		fmt.Println("CV data is available via the MCP server. Run: vibecheck mcp-server")
	},
}

func init() {
	// Add mcp-server command
	rootCmd.AddCommand(mcpServerCmd)

	// Add cv display command
	rootCmd.AddCommand(cvCmd)
}
