#!/bin/bash
#
# Claude Code Statusline Script
# https://code.claude.com/docs/en/statusline
#

# Bash strict mode for better error handling
set -euo pipefail

# Global variables
INPUT_JSON=""
HAS_JQ=0
GIT_BRANCH=""
GIT_STAGED=0
GIT_UNSTAGED=0
GIT_UNTRACKED=0
CURRENT_DIR=""
MODEL_NAME=""
CC_VERSION=""
OUTPUT_STYLE=""
COST_USD=""
COST_PER_HOUR=""
CONTEXT_USED_PCT=""

# Check for jq availability
if command -v jq >/dev/null 2>&1; then
    HAS_JQ=1
fi

# Read input safely
if ! INPUT_JSON=$(cat); then
    echo "Error: Failed to read input" >&2
    exit 1
fi

# ===================================================================
# COLOR FUNCTIONS
# ===================================================================

# Color configuration
USE_COLOR=1
[[ -n "${NO_COLOR:-}" ]] && USE_COLOR=0

# Color utility functions
color_code() {
    [[ "$USE_COLOR" -eq 1 ]] && printf '\033[%sm' "$1"
}

reset_color() {
    [[ "$USE_COLOR" -eq 1 ]] && printf '\033[0m'
}

# Modern color palette
dir_color() { color_code '38;5;117'; }      # sky blue, bold
model_color() { color_code '38;5;147'; }    # light purple, bold
cc_version_color() { color_code '38;2;0;130;255'; }  # blue RGB(0,130,255), bold
style_color() { color_code '38;5;245'; }    # gray
git_color() { color_code '38;5;150'; }      # soft green, bold
git_modified_color() { color_code '38;5;178'; }  # yellow (staged/unstaged)
git_untracked_color() { color_code '38;5;39'; }  # blue (untracked)
usage_color() { color_code '38;5;189'; }    # lavender
cost_color() { color_code '38;5;222'; }     # light gold
burn_color() { color_code '38;5;220'; }     # bright gold
icon_color() { color_code '38;2;242;242;242'; }  # light white RGB(242,242,242)

# ===================================================================
# GIT FUNCTIONS
# ===================================================================

# 取得 git branch、已修改檔案數、新增檔案數
get_git_info() {
    if git rev-parse --git-dir >/dev/null 2>&1; then
        GIT_BRANCH=$(git branch --show-current 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)

        # 透過 git status --porcelain 計算已暫存、已修改、新增檔案數
        local status_output
        status_output=$(git status --porcelain 2>/dev/null) || return

        if [[ -n "$status_output" ]]; then
            # 第一欄非空非? = staged，第二欄非空非? = unstaged，?? = untracked
            GIT_STAGED=$(echo "$status_output" | grep -c '^[MADRC]' || true)
            GIT_UNSTAGED=$(echo "$status_output" | grep -c '^.[MADRC]' || true)
            GIT_UNTRACKED=$(echo "$status_output" | grep -c '^??' || true)
        fi
    fi
}

# ===================================================================
# COST AND USAGE FUNCTIONS
# ===================================================================

# Extract cost and usage data
get_cost_usage_info() {
    local cost_usd="" total_duration_ms=""

    # Extract cost data from Claude Code input
    if [[ "$HAS_JQ" -eq 1 ]]; then
        cost_usd=$(echo "$INPUT_JSON" | jq -r '.cost.total_cost_usd // empty' 2>/dev/null)
        total_duration_ms=$(echo "$INPUT_JSON" | jq -r '.cost.total_duration_ms // empty' 2>/dev/null)
    else
        # Bash fallback for cost extraction
        cost_usd=$(echo "$INPUT_JSON" | grep -o '"total_cost_usd"[[:space:]]*:[[:space:]]*[0-9.]*' | sed 's/.*:[[:space:]]*\([0-9.]*\).*/\1/' 2>/dev/null)
        total_duration_ms=$(echo "$INPUT_JSON" | grep -o '"total_duration_ms"[[:space:]]*:[[:space:]]*[0-9]*' | sed 's/.*:[[:space:]]*\([0-9]*\).*/\1/' 2>/dev/null)
    fi

    COST_USD="$cost_usd"

    # Calculate burn rate ($/hour) from cost and duration
    if [[ -n "$cost_usd" && -n "$total_duration_ms" && "$total_duration_ms" -gt 0 ]]; then
        COST_PER_HOUR=$(echo "$cost_usd $total_duration_ms" | awk '{printf "%.2f", $1 * 3600000 / $2}')
    fi

    # Extract context window usage
    if [[ "$HAS_JQ" -eq 1 ]]; then
        CONTEXT_USED_PCT=$(echo "$INPUT_JSON" | jq -r '.context_window.used_percentage // empty' 2>/dev/null)
    else
        CONTEXT_USED_PCT=$(echo "$INPUT_JSON" | grep -o '"used_percentage"[[:space:]]*:[[:space:]]*[0-9.]*' | sed 's/.*:[[:space:]]*\([0-9.]*\).*/\1/' 2>/dev/null)
    fi
}

# Extract basic information using jq or fallback
get_basic_info() {
    local current_dir model_name cc_version output_style

    if [[ "$HAS_JQ" -eq 1 ]]; then
        current_dir=$(echo "$INPUT_JSON" | jq -r '.workspace.current_dir // .cwd // "unknown"' 2>/dev/null | sed "s|^$HOME|~|g")
        model_name=$(echo "$INPUT_JSON" | jq -r '.model.display_name // "Claude"' 2>/dev/null)
        cc_version=$(echo "$INPUT_JSON" | jq -r '.version // ""' 2>/dev/null)
        output_style=$(echo "$INPUT_JSON" | jq -r '.output_style.name // ""' 2>/dev/null)
    else
        # Bash fallback for JSON extraction
        current_dir=$(echo "$INPUT_JSON" | grep -o '"workspace"[[:space:]]*:[[:space:]]*{[^}]*"current_dir"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"current_dir"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | sed 's/\\\\/\//g' 2>/dev/null)

        # Fall back to cwd if workspace extraction failed
        if [[ -z "$current_dir" || "$current_dir" == "null" ]]; then
            current_dir=$(echo "$INPUT_JSON" | grep -o '"cwd"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"cwd"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | sed 's/\\\\/\//g' 2>/dev/null)
        fi

        [[ -z "$current_dir" ]] && current_dir="unknown"
        current_dir=$(echo "$current_dir" | sed "s|^$HOME|~|g")

        model_name=$(echo "$INPUT_JSON" | grep -o '"model"[[:space:]]*:[[:space:]]*{[^}]*"display_name"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"display_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' 2>/dev/null)
        [[ -z "$model_name" ]] && model_name="Claude"

        cc_version=$(echo "$INPUT_JSON" | grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' 2>/dev/null)
        output_style=$(echo "$INPUT_JSON" | grep -o '"output_style"[[:space:]]*:[[:space:]]*{[^}]*"name"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' 2>/dev/null)
    fi

    # Export variables for global use
    CURRENT_DIR="$current_dir"
    MODEL_NAME="$model_name"
    CC_VERSION="$cc_version"
    OUTPUT_STYLE="$output_style"
}

# ===================================================================
# MAIN EXECUTION
# ===================================================================

# Render statusline
render_line() {
    local sep='    '

    printf '%s󰝰%s %s%s%s' "$(icon_color)" "$(reset_color)" "$(dir_color)" "$CURRENT_DIR" "$(reset_color)"

    if [[ -n "$GIT_BRANCH" ]]; then
        printf '%s' "$sep"
        printf '%s%s %s%s%s' "$(icon_color)" "$(reset_color)" "$(git_color)" "$GIT_BRANCH" "$(reset_color)"

        # 顮示已暫存檔案數（與 p10k +N 一致）
        if [[ "$GIT_STAGED" -gt 0 ]]; then
            printf ' %s+%d%s' "$(git_modified_color)" "$GIT_STAGED" "$(reset_color)"
        fi

        # 顮示已修改未暫存檔案數（與 p10k !N 一致）
        if [[ "$GIT_UNSTAGED" -gt 0 ]]; then
            printf ' %s!%d%s' "$(git_modified_color)" "$GIT_UNSTAGED" "$(reset_color)"
        fi

        # 顮示新增 untracked 檔案數（與 p10k ?N 一致）
        if [[ "$GIT_UNTRACKED" -gt 0 ]]; then
            printf ' %s?%d%s' "$(git_untracked_color)" "$GIT_UNTRACKED" "$(reset_color)"
        fi
    fi

    printf '%s' "$sep"
    printf '%s󰚩%s %s%s%s' "$(icon_color)" "$(reset_color)" "$(model_color)" "$MODEL_NAME" "$(reset_color)"

    if [[ -n "$CC_VERSION" && "$CC_VERSION" != "null" ]]; then
        printf '%s' "$sep"
        printf '%s%s %sv%s%s' "$(icon_color)" "$(reset_color)" "$(cc_version_color)" "$CC_VERSION" "$(reset_color)"
    fi

    # Context window usage (default to 0 when not yet available)
    [[ -z "$CONTEXT_USED_PCT" ]] && CONTEXT_USED_PCT=0
    {
        local int_pct="${CONTEXT_USED_PCT%%.*}"
        [[ -z "$int_pct" ]] && int_pct=0

        local filled_count=$(( int_pct / 10 ))
        (( filled_count > 10 )) && filled_count=10
        local empty_count=$(( 10 - filled_count ))

        local skulls=""
        local i
        for ((i=0; i<filled_count; i++)); do
            skulls+="󰚌 "
        done
        for ((i=0; i<empty_count; i++)); do
            skulls+="󰯈 "
        done

        printf '%s' "$sep"
        printf '\033[1mStress: %s%s%s' "$(usage_color)" "$skulls" "$(reset_color)"
    }

    # Claude API Usage from main.go
    local api_usage
    api_usage=$(echo "$INPUT_JSON" | "${BASH_SOURCE[0]%/*}/get-claude-usage" 2>/dev/null)
    if [[ -n "$api_usage" ]]; then
        printf '%s' "$sep"
        printf '%s' "$api_usage"
    fi
}

# Main function to orchestrate statusline generation
main() {
    # Initialize all data
    get_basic_info
    get_git_info
    get_cost_usage_info

    # Render statusline
    render_line
    printf '\n'
}

# Execute main function
main
