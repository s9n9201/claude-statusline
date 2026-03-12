package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	DefaultVersion = "2.1.20"
	AnthropicBeta  = "oauth-2025-04-20"
)

// SecurityOutput represents the JSON structure stored in Keychain
type SecurityOutput struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// UsageResponse represents the API response from Anthropic
type UsageResponse struct {
	FiveHour *UsageData `json:"five_hour"`
	SevenDay *UsageData `json:"seven_day"`
}

type UsageData struct {
	Utilization float64    `json:"utilization"`
	ResetsAt    *time.Time `json:"resets_at"`
}

// InputData represents the input JSON from Claude Code
type InputData struct {
	Version string `json:"version"`
}

// Config holds the application configuration
type Config struct {
	KeychainService string
	APIURL          string
}

// loadConfig reads from .env file using godotenv
func loadConfig() Config {
	// Load .env file (if exists, it sets process env vars)
	_ = godotenv.Load()

	// Read from Environment Variables
	return Config{
		KeychainService: os.Getenv("KEYCHAIN_SERVICE"),
		APIURL:          os.Getenv("API_URL"),
	}
}

func getCredentials(serviceName string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", serviceName, "-w")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get credentials: %w", err)
	}

	var credentials SecurityOutput
	if err := json.Unmarshal(output, &credentials); err != nil {
		return "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	if credentials.ClaudeAiOauth.AccessToken == "" {
		return "", fmt.Errorf("access token not found")
	}

	return credentials.ClaudeAiOauth.AccessToken, nil
}

func fetchUsage(token string, version string, config Config) (*UsageResponse, error) {
	req, err := http.NewRequest("GET", config.APIURL, nil)
	if err != nil {
		return nil, err
	}

	// Use default version if not provided
	if version == "" {
		version = DefaultVersion
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-code/"+version)
	req.Header.Set("anthropic-beta", AnthropicBeta)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var rawJSON json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawJSON); err != nil {
		return nil, err
	}

	// Output raw API response as formatted JSON
	var prettyJSON map[string]interface{}
	json.Unmarshal(rawJSON, &prettyJSON)
	formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
	fmt.Println(string(formatted))

	var usage UsageResponse
	if err := json.Unmarshal(rawJSON, &usage); err != nil {
		return nil, err
	}

	return &usage, nil
}

func formatOutput(usage *UsageResponse) string {
	if usage == nil {
		return ""
	}

	var parts []string

	if usage.FiveHour != nil {
		if usage.FiveHour.ResetsAt != nil {
			resetTime := usage.FiveHour.ResetsAt.Local().Format("15:04")
			parts = append(parts, fmt.Sprintf("5h: %.0f%% (Reset %s)", usage.FiveHour.Utilization, resetTime))
		} else {
			parts = append(parts, fmt.Sprintf("5h: %.0f%%", usage.FiveHour.Utilization))
		}
	}

	return strings.Join(parts, " | ")
}

func main() {
	// 1. Load Configuration
	config := loadConfig()

	if config.KeychainService == "" || config.APIURL == "" {
		fmt.Printf("Error: Missing required configuration. Please check KEYCHAIN_SERVICE and API_URL in .env or environment variables.\n")
		return
	}

	// Try to read version from stdin with a short timeout to prevent blocking in IDEs
	// This approach is more robust than checking file mode bits
	inputChan := make(chan InputData, 1)
	go func() {
		var input InputData
		// We use a new scanner/decoder to check if there is actual content
		// If Stdin is empty or blocking, this goroutine will just hang (which is fine, main will timeout)
		if err := json.NewDecoder(os.Stdin).Decode(&input); err == nil {
			inputChan <- input
		}
		close(inputChan)
	}()

	var input InputData
	select {
	case data := <-inputChan:
		input = data
	case <-time.After(100 * time.Millisecond):
		// Timeout reached, assume no input provided
		// Proceeding with zero-valued input (empty Version)
	}

	token, err := getCredentials(config.KeychainService)
	if err != nil {
		fmt.Printf("Error getting credentials: %v\n", err)
		return
	}

	usage, err := fetchUsage(token, input.Version, config)
	if err != nil {
		fmt.Printf("Error fetching usage: %v\n", err)
		return
	}

	fmt.Print(formatOutput(usage))
}
