package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/kfreiman/vibecheck/internal/mcp"
	"github.com/samber/slog-zerolog"
	"github.com/spf13/cobra"
)

var cmdLogger = slog.New(slogzerolog.Option{}.NewZerologHandler())

// mcpServerCmd represents the mcp-server command
var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Start an MCP server",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		cmdLogger.InfoContext(ctx, "mcp server starting",
			"port", 8080,
			"endpoints", []string{"/mcp", "/sse", "/health/live", "/health/ready"},
		)

		err := mcp.StartMCPServer()
		if err != nil {
			cmdLogger.ErrorContext(ctx, "failed to start MCP server",
				"error", err,
			)
			os.Exit(1)
		}
	},
}

// cvCmd represents the cv command
var cvCmd = &cobra.Command{
	Use:   "cv",
	Short: "Display example CV data in markdown format",
	Long: `Display example CV/resume data in markdown format.

This command displays CV data in markdown format via the MCP server.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		cmdLogger.InfoContext(ctx, "cv command executed - MCP server required",
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
