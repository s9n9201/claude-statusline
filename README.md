# claude-statusline

A rich, informative status line for [Claude Code](https://code.claude.com) that displays workspace context, git status, API usage, and more — right in your terminal.

![claude-statusline screenshot](statusline-image.png)

## Features

- **Repository path** — current working directory with `~` shorthand
- **Git branch** — branch name with staged (`+N`), unstaged (`!N`), and untracked (`?N`) file counts
- **API usage (Life)** — 5-hour usage visualized as a heart bar, with reset time countdown
- **Context stress** — context window utilization displayed as a 10-segment skull bar
- **Model & version** — active Claude model and Claude Code version
- **Smart caching** — API responses cached for 5 minutes to avoid rate limits
- **Color safe** — respects `NO_COLOR` environment variable

## Requirements

- **macOS** (uses Keychain for OAuth token storage)
- **Go 1.25+** (for building the usage binary)
- **Bash 4.0+**
- **Nerd Font** — required for icons to render correctly. We recommend using a **non-Mono** variant (e.g. `JetBrainsMono Nerd Font` instead of `JetBrainsMono Nerd Font Mono`) as Mono variants render double-width icons at single width, causing alignment issues. You can browse and download fonts at [nerdfonts.com](https://www.nerdfonts.com/).
- **jq** (optional) — used for JSON parsing; falls back to grep/sed if unavailable

## Installation

1. Clone and build:

   ```bash
   git clone https://github.com/s9n9201/claude-statusline.git
   cd claude-statusline
   go build -o get-claude-usage main.go
   ```

2. Copy `statusline.sh` and `get-claude-usage` to `~/.claude/`:

   ```bash
   cp statusline.sh get-claude-usage ~/.claude/
   ```

   > **Note:** `get-claude-usage` must be in the same directory as `statusline.sh`.

3. Configure Claude Code to use the status line. Add to your Claude Code settings (`~/.claude/settings.json`):

   ```json
   {
     "statusline": "~/.claude/statusline.sh"
   }
   ```

   See the [Claude Code statusline docs](https://code.claude.com/docs/en/statusline) for more details.

## How It Works

Claude Code pipes a JSON payload via stdin to `statusline.sh` on each render cycle. The shell script extracts workspace info, git status, context window usage, and cost data. It then delegates API usage fetching to the `get-claude-usage` Go binary, which retrieves an OAuth token from macOS Keychain, calls the Anthropic usage API (with a 5-minute file cache at `/tmp/claude_usage_cache.json`), and returns a heart bar visualization. The shell script composes everything into a single colored output line.

## Configuration

| Environment Variable | Description |
|---|---|
| `NO_COLOR` | Set to any value to disable color output |

Cache file location: `/tmp/claude_usage_cache.json` (auto-created, 5-minute TTL)