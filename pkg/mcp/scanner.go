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

	// log.Info().Str("url", s.options.HTTPUrl).Msg("Connecting to MCP server")

	// Parse and validate the URL
	parsedURL, err := url.Parse(s.options.HTTPUrl)
	if err != nil {
		return fmt.Errorf("invalid HTTP URL: %w", err)
	}

	// Initialize MCP connection
	if err := s.initialize(ctx, parsedURL.String()); err != nil {
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// log.Info().Msg("Successfully connected to MCP server")

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
	// log.Info().Msg("Listing available tools...")

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

	// log.Info().Int("count", len(tools)).Msg("Retrieved tools")
	return tools
}

// listPrompts retrieves all available prompts from the MCP server
func (s *Scanner) listPrompts(ctx context.Context, serverURL string) []PromptInfo {
	// log.Info().Msg("Listing available prompts...")

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

	// log.Info().Int("count", len(prompts)).Msg("Retrieved prompts")
	return prompts
}

// listResources retrieves all available resources from the MCP server
func (s *Scanner) listResources(ctx context.Context, serverURL string) []ResourceInfo {
	// log.Info().Msg("Listing available resources...")

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

	// 	log.Info().Int("count", len(resources)).Msg("Retrieved resources")
	return resources
}

// sendRequest sends a JSON-RPC request and expects a response
func (s *Scanner) sendRequest(ctx context.Context, serverURL string, request JSONRPCRequest, response *JSONRPCResponse) error {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// log.Debug().Str("method", request.Method).Bytes("body", requestBody).Msg("Sending MCP request")

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
		// log.Debug().Str("sessionID", s.sessionID).Msg("Including session ID in request")
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
		// log.Debug().Str("sessionID", sessionID).Msg("Captured session ID")
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// log.Debug().Str("method", request.Method).Bytes("response", responseBody).Msg("Received MCP response")

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

// displayResults prints the retrieved data in a clean format with injection detection
func (s *Scanner) displayResults(tools []ToolInfo, prompts []PromptInfo, resources []ResourceInfo) {
	fmt.Println("=== MCP SERVER SCAN RESULTS WITH INJECTION DETECTION ===")

	// Analyze tools for injection
	if len(tools) > 0 {
		fmt.Println("\n📋 TOOLS:")
		for i, tool := range tools {
			fmt.Printf("\n%d. %s\n", i+1, tool.Name)
			fmt.Printf("   Description: %s\n", tool.Description)

			// Analyze for injection
			analysis := s.analyzeForInjection(tool.Name, tool.Description, "tool")
			s.displayAnalysisResult(analysis)
		}
	}

	// Analyze prompts for injection
	if len(prompts) > 0 {
		fmt.Println("\n💬 PROMPTS:")
		for i, prompt := range prompts {
			fmt.Printf("\n%d. %s\n", i+1, prompt.Name)
			fmt.Printf("   Description: %s\n", prompt.Description)

			// Analyze for injection
			analysis := s.analyzeForInjection(prompt.Name, prompt.Description, "prompt")
			s.displayAnalysisResult(analysis)
		}
	}

	// Analyze resources for injection
	if len(resources) > 0 {
		fmt.Println("\n📁 RESOURCES:")
		for i, resource := range resources {
			fmt.Printf("\n%d. %s\n", i+1, resource.Name)
			fmt.Printf("   URI: %s\n", resource.URI)
			fmt.Printf("   Description: %s\n", resource.Description)

			// Analyze for injection
			analysis := s.analyzeForInjection(resource.Name, resource.Description, "resource")
			s.displayAnalysisResult(analysis)
		}
	}

	if len(tools) == 0 && len(prompts) == 0 && len(resources) == 0 {
		fmt.Println("\n❌ No tools, prompts, or resources found.")
	}

	fmt.Println("\n=== END SCAN ===")
}

// analyzeForInjection sends tool metadata to Python server for analysis
func (s *Scanner) analyzeForInjection(name, description, itemType string) map[string]interface{} {
	// Prepare request data
	requestData := map[string]interface{}{
		"name":        name,
		"description": description,
		"type":        itemType,
	}

	// Use json.Encoder with SetEscapeHTML(false) to preserve original characters
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(requestData); err != nil {
		return map[string]interface{}{"error": "Failed to marshal request"}
	}

	// Remove the trailing newline that encoder.Encode adds
	jsonData := bytes.TrimSpace(buf.Bytes())

	log.Info().Msgf("Request data: %v", requestData)
	log.Info().Msgf("Sending request to Python server: %s", string(jsonData))

	// Make HTTP request to Python server
	resp, err := s.httpClient.Post("http://localhost:5001/analyze", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("HTTP request failed: %v", err)}
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{"error": "Failed to read response"}
	}

	log.Info().Msgf("Received response from Python server: %s", string(body))

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"error": "Failed to parse response"}
	}

	log.Info().Msgf("Parsed response from Python server: %v", result)
	return result
}

// displayAnalysisResult displays injection *and* ban-code analysis results
func (s *Scanner) displayAnalysisResult(analysis map[string]interface{}) {
	if errorMsg, exists := analysis["error"]; exists {
		fmt.Printf("     ⚠️  Analysis Error: %v\n", errorMsg)
		return
	}

	detection, ok := analysis["detection"].(map[string]interface{})
	if !ok {
		fmt.Printf("     ⚠️  Invalid detection result\n")
		return
	}

	// ---- Prompt-Injection verdict ----
	if isInjection, _ := detection["is_injection"].(bool); isInjection {
		conf, _ := detection["confidence"].(float64)
		risk, _ := detection["risk_level"].(string)
		riskEmoji := map[string]string{"HIGH": "🚨", "MEDIUM": "⚠️"}[risk]
		if riskEmoji == "" {
			riskEmoji = "🚨"
		}
		fmt.Printf("     %s INJECTION DETECTED (Confidence: %.2f) - %s RISK\n", riskEmoji, conf, risk)
	} else {
		conf, _ := detection["confidence"].(float64)
		fmt.Printf("     ✅ NO INJECTION DETECTED (Confidence: %.2f)\n", conf)
	}

	// ---- Ban-Code / secrets verdict ----
	if bc, ok := analysis["code_detection"].(map[string]interface{}); ok {
		if isCode, _ := bc["is_code"].(bool); isCode {
			conf, _ := bc["confidence"].(float64)
			reason, _ := bc["reason"].(string)
			pattern, _ := bc["pattern"].(string)
			fmt.Printf("     🔑 BANNED CODE/SECRET DETECTED (Confidence: %.2f) - %s %s\n", conf, reason, pattern)
		} else {
			conf, _ := bc["confidence"].(float64)
			fmt.Printf("     ✅ No banned code detected (Confidence: %.2f)\n", conf)
		}
	}
}
