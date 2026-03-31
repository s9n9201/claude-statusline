package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultVersion  = "2.1.87"
	AnthropicBeta   = "oauth-2025-04-20"
	KeychainService = "Claude Code-credentials"
	ApiUrl          = "https://api.anthropic.com/api/oauth/usage"
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

func fetchUsage(token string, version string) (*UsageResponse, error) {
	req, err := http.NewRequest("GET", ApiUrl, nil)
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
	//var prettyJSON map[string]interface{}
	//json.Unmarshal(rawJSON, &prettyJSON)
	//formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
	//fmt.Println(string(formatted))

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
		//green := "\033[1;38;2;0;175;80m"
		//orange := "\033[1;38;2;255;115;0m"
		red := "\033[1;38;2;255;40;0m"
		reset := "\033[0m"
		heart := "\U000F02D1"
		halfHeart := "\U000F06DE"
		brokeHeart := "\U000F02D5"

		utilization := usage.FiveHour.Utilization
		color := red

		brokenCount := int(utilization) / 10
		halfCount := 0
		if int(utilization)%10 >= 5 || (utilization > 0 && utilization < 5) {
			halfCount = 1
		}
		fullCount := 10 - brokenCount - halfCount

		bar := make([]string, 10)
		for i := 0; i < 10; i++ {
			if i < fullCount {
				bar[i] = heart
			} else if i < fullCount+halfCount {
				bar[i] = halfHeart
			} else {
				bar[i] = brokeHeart
			}
		}
		barStr := strings.Join(bar, " ")

		if usage.FiveHour.ResetsAt != nil {
			resetTime := usage.FiveHour.ResetsAt.Local().Format("15:04 PM")
			parts = append(parts, fmt.Sprintf("\033[1mLife: %s%s%s  (%s)", color, barStr, reset, resetTime))
		} else {
			parts = append(parts, fmt.Sprintf("\033[1mLife: %s%s%s", color, barStr, reset))
		}
	}

	return strings.Join(parts, " | ")
}

func readCache() (*UsageResponse, error) {
	cacheFile := filepath.Join(os.TempDir(), "claude_usage_cache.json")
	if info, err := os.Stat(cacheFile); err == nil && time.Since(info.ModTime()) < 5*time.Minute {
		if data, err := os.ReadFile(cacheFile); err == nil {
			var usage UsageResponse
			if err := json.Unmarshal(data, &usage); err == nil {
				return &usage, nil
			}
		}
	}
	return nil, fmt.Errorf("cache miss or expired")
}

func writeCache(usage *UsageResponse) {
	cacheFile := filepath.Join(os.TempDir(), "claude_usage_cache.json")

	data, err := json.Marshal(usage)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to marshal cache data: %v\n", err)
		return
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		// 使用 _, _ = 來明確放棄 Fprintf 本身的錯誤回傳值，平息 IDE 警告
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to write cache: %v\n", err)
	}
}

func readInputWithTimeout(timeout time.Duration) InputData {
	inputChan := make(chan InputData, 1)
	go func() {
		var input InputData
		// 如果 Stdin 沒有資料，這個 Goroutine 會阻塞，但在背景不影響主程式
		if err := json.NewDecoder(os.Stdin).Decode(&input); err == nil {
			inputChan <- input
		}
		close(inputChan)
	}()

	select {
	case data := <-inputChan:
		return data
	case <-time.After(timeout):
		// 超時未收到資料，回傳空的結構（代表使用預設版本）
		return InputData{}
	}
}

func main() {
	// 1. 檢查快取 (5 分鐘內有效)
	if usage, err := readCache(); err == nil && usage != nil {
		// 命中快取，直接輸出結果並結束，不呼叫 API
		fmt.Println(formatOutput(usage))
		return
	}

	// 2. 嘗試讀取 Stdin 中外部傳來的版本參數
	input := readInputWithTimeout(100 * time.Millisecond)

	// 3. 快取無效或過期，從 Keychain 拿憑證並呼叫 API
	token, err := getCredentials(KeychainService)
	if err != nil {
		fmt.Printf("Error getting credentials: %v\n", err)
		return
	}

	usage, err := fetchUsage(token, input.Version)
	if err != nil {
		fmt.Printf("Error fetching usage: %v\n", err)
		return
	}

	// 5. 把剛剛取得的新結果存入快取，供未來 5 分鐘使用 (並處理錯誤)
	writeCache(usage)

	fmt.Println(formatOutput(usage))
}
