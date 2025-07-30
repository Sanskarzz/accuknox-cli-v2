package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Current MCP protocol version (2025-06-18)
const MCPProtocolVersion = "2025-06-18"

// Scanner handles MCP server scanning operations
type Scanner struct {
	options    *ScanOptions
	httpClient *http.Client
	requestID  int
	sessionID  string
}

// New creates a new MCP scanner with the given options
func New(options *ScanOptions) *Scanner {
	return &Scanner{
		options: options,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		requestID: 1,
	}
}

// Scan connects to the MCP server and retrieves tools, prompts, and resources
func (s *Scanner) Scan() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info().Str("url", s.options.HTTPUrl).Msg("Connecting to MCP server")

	// Parse and validate the URL
	parsedURL, err := url.Parse(s.options.HTTPUrl)
	if err != nil {
		return fmt.Errorf("invalid HTTP URL: %w", err)
	}

	// Initialize MCP connection
	if err := s.initialize(ctx, parsedURL.String()); err != nil {
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	log.Info().Msg("Successfully connected to MCP server")

	// List all data concurrently (continue on errors to get partial results)
	tools := s.listTools(ctx, parsedURL.String())
	prompts := s.listPrompts(ctx, parsedURL.String())
	resources := s.listResources(ctx, parsedURL.String())

	// Display results
	s.displayResults(tools, prompts, resources)

	return nil
}

// initialize performs MCP protocol initialization
func (s *Scanner) initialize(ctx context.Context, serverURL string) error {
	initParams := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    ClientCapabilities{}, // Minimal capabilities
		ClientInfo: Implementation{
			Name:    "knoxctl-mcp-scanner",
			Version: "1.0.0",
		},
	}

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "initialize",
		Params:  initParams,
	}

	var response JSONRPCResponse
	if err := s.sendRequest(ctx, serverURL, request, &response); err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialize error: %s (code: %d)", response.Error.Message, response.Error.Code)
	}

	// Send initialized notification
	notification := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	// Send notification (don't fail on errors)
	if err := s.sendNotification(ctx, serverURL, notification); err != nil {
		log.Warn().Err(err).Msg("Failed to send initialized notification")
	}

	return nil
}

// listTools retrieves all available tools from the MCP server
func (s *Scanner) listTools(ctx context.Context, serverURL string) []ToolInfo {
	log.Info().Msg("Listing available tools...")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "tools/list",
	}

	var response JSONRPCResponse
	if err := s.sendRequest(ctx, serverURL, request, &response); err != nil {
		log.Warn().Err(err).Msg("Failed to list tools")
		return []ToolInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Tools list error")
		return []ToolInfo{}
	}

	// Parse the result
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal tools result")
		return []ToolInfo{}
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal tools result")
		return []ToolInfo{}
	}

	var tools []ToolInfo
	for _, tool := range listResult.Tools {
		tools = append(tools, ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	log.Info().Int("count", len(tools)).Msg("Retrieved tools")
	return tools
}

// listPrompts retrieves all available prompts from the MCP server
func (s *Scanner) listPrompts(ctx context.Context, serverURL string) []PromptInfo {
	log.Info().Msg("Listing available prompts...")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "prompts/list",
	}

	var response JSONRPCResponse
	if err := s.sendRequest(ctx, serverURL, request, &response); err != nil {
		log.Warn().Err(err).Msg("Failed to list prompts")
		return []PromptInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Prompts list error")
		return []PromptInfo{}
	}

	// Parse the result
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal prompts result")
		return []PromptInfo{}
	}

	var listResult ListPromptsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal prompts result")
		return []PromptInfo{}
	}

	var prompts []PromptInfo
	for _, prompt := range listResult.Prompts {
		prompts = append(prompts, PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
		})
	}

	log.Info().Int("count", len(prompts)).Msg("Retrieved prompts")
	return prompts
}

// listResources retrieves all available resources from the MCP server
func (s *Scanner) listResources(ctx context.Context, serverURL string) []ResourceInfo {
	log.Info().Msg("Listing available resources...")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      s.nextRequestID(),
		Method:  "resources/list",
	}

	var response JSONRPCResponse
	if err := s.sendRequest(ctx, serverURL, request, &response); err != nil {
		log.Warn().Err(err).Msg("Failed to list resources")
		return []ResourceInfo{}
	}

	if response.Error != nil {
		log.Warn().Str("error", response.Error.Message).Msg("Resources list error")
		return []ResourceInfo{}
	}

	// Parse the result
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal resources result")
		return []ResourceInfo{}
	}

	var listResult ListResourcesResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal resources result")
		return []ResourceInfo{}
	}

	var resources []ResourceInfo
	for _, resource := range listResult.Resources {
		resources = append(resources, ResourceInfo{
			Name:        resource.Name,
			Description: resource.Description,
			URI:         resource.URI,
		})
	}

	log.Info().Int("count", len(resources)).Msg("Retrieved resources")
	return resources
}

// sendRequest sends a JSON-RPC request and expects a response
func (s *Scanner) sendRequest(ctx context.Context, serverURL string, request JSONRPCRequest, response *JSONRPCResponse) error {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Debug().Str("method", request.Method).Bytes("body", requestBody).Msg("Sending MCP request")

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers per MCP spec
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Include session ID if we have one
	if s.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.sessionID)
		log.Debug().Str("sessionID", s.sessionID).Msg("Including session ID in request")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Capture session ID from response headers
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		s.sessionID = sessionID
		log.Debug().Str("sessionID", sessionID).Msg("Captured session ID")
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().Str("method", request.Method).Bytes("response", responseBody).Msg("Received MCP response")

	// Handle Server-Sent Events format
	responseStr := string(responseBody)
	if strings.HasPrefix(responseStr, "event:") {
		jsonData, err := s.parseSSEResponse(responseStr)
		if err != nil {
			return fmt.Errorf("failed to parse SSE response: %w", err)
		}
		responseBody = []byte(jsonData)
	}

	return json.Unmarshal(responseBody, response)
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (s *Scanner) sendNotification(ctx context.Context, serverURL string, notification JSONRPCRequest) error {
	requestBody, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if s.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.sessionID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Log any errors but don't fail
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().Int("status", resp.StatusCode).Bytes("body", body).Msg("Notification returned error status")
	}

	return nil
}

// nextRequestID returns the next request ID
func (s *Scanner) nextRequestID() int {
	id := s.requestID
	s.requestID++
	return id
}

// parseSSEResponse extracts JSON data from Server-Sent Events format
func (s *Scanner) parseSSEResponse(sseData string) (string, error) {
	lines := strings.Split(sseData, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), nil
		}
	}
	return "", fmt.Errorf("no data field found in SSE response")
}

// displayResults prints the retrieved data in a formatted way
func (s *Scanner) displayResults(tools []ToolInfo, prompts []PromptInfo, resources []ResourceInfo) {
	fmt.Println("\n=== MCP Server Scan Results ===")

	fmt.Printf("\n🔧 Tools (%d):\n", len(tools))
	if len(tools) == 0 {
		fmt.Println("  No tools found")
	} else {
		for i, tool := range tools {
			fmt.Printf("  %d. %s\n", i+1, tool.Name)
			if tool.Description != "" {
				fmt.Printf("     Description: %s\n", tool.Description)
			}
			fmt.Println()
		}
	}

	fmt.Printf("💬 Prompts (%d):\n", len(prompts))
	if len(prompts) == 0 {
		fmt.Println("  No prompts found")
	} else {
		for i, prompt := range prompts {
			fmt.Printf("  %d. %s\n", i+1, prompt.Name)
			if prompt.Description != "" {
				fmt.Printf("     Description: %s\n", prompt.Description)
			}
			fmt.Println()
		}
	}

	fmt.Printf("📁 Resources (%d):\n", len(resources))
	if len(resources) == 0 {
		fmt.Println("  No resources found")
	} else {
		for i, resource := range resources {
			fmt.Printf("  %d. %s\n", i+1, resource.Name)
			if resource.Description != "" {
				fmt.Printf("     Description: %s\n", resource.Description)
			}
			if resource.URI != "" {
				fmt.Printf("     URI: %s\n", resource.URI)
			}
			fmt.Println()
		}
	}

	fmt.Println("=== Scan Complete ===")
}
