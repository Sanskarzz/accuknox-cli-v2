package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/mcp"
	"github.com/spf13/cobra"
)

var mcpScanOpts mcp.ScanOptions

var mcpScanCmd = &cobra.Command{
	Use:   "mcp-scan",
	Short: "Scan MCP servers for prompt injections in tool descriptions",
	Long: `
Connect to an MCP (Model Context Protocol) server and analyze tools, prompts, and resources 
for potential prompt injection vulnerabilities in their descriptions.

This command connects to MCP servers via HTTP and retrieves information about available
tools, prompts, and resources. The retrieved data can then be analyzed for security issues.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mcpScanOpts.HTTPUrl == "" {
			return fmt.Errorf("--http-url flag is required")
		}

		scanner := mcp.New(&mcpScanOpts)

		if err := scanner.Scan(); err != nil {
			fmt.Printf("Error scanning MCP server: %v\n", err)
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpScanCmd)

	mcpScanCmd.Flags().StringVar(&mcpScanOpts.HTTPUrl, "http-url", "", "URL of the MCP server exposed via HTTP (required)")
	mcpScanCmd.MarkFlagRequired("http-url")
}
