package cmd

import (
	"fmt"
	"log"

	"github.com/kfreiman/vibecheck/internal/mcp"
	"github.com/spf13/cobra"
)

// mcpServerCmd represents the mcp-server command
var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Start an MCP server",
	Run: func(cmd *cobra.Command, args []string) {

		err := mcp.StartMCPServer()
		if err != nil {
			log.Fatalf("Failed to start MCP server: %v", err)
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
		fmt.Println("CV data is available via the MCP server. Run: vibecheck mcp-server")
	},
}

func init() {
	// Add mcp-server command
	rootCmd.AddCommand(mcpServerCmd)

	// Add cv display command
	rootCmd.AddCommand(cvCmd)
}
