//go:build darwin

package notifier

import (
	"os"
	"os/exec"
	"strings"

	"github.com/777genius/claude-notifications/internal/logging"
)

// TmuxPaneInfo contains the current tmux pane details for click-to-focus navigation
type TmuxPaneInfo struct {
	SessionName string // tmux session name
	WindowIndex string // window index within session
	PaneIndex   string // pane index within window
	Target      string // full target string like "session:0.1"
	Client      string // tmux client name (e.g., /dev/ttys000)
}

// GetCurrentTmuxPane detects if we're running inside tmux and returns the current pane info.
// Returns nil if not in a tmux session.
func GetCurrentTmuxPane() *TmuxPaneInfo {
	// Check if we're inside tmux
	if os.Getenv("TMUX") == "" {
		return nil
	}

	// Get the full pane target in format "session_name:window_index.pane_index"
	cmd := exec.Command("tmux", "display-message", "-p", "#{session_name}:#{window_index}.#{pane_index}")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug("Failed to get tmux pane info: %v", err)
		return nil
	}

	target := strings.TrimSpace(string(output))
	if target == "" {
		return nil
	}

	// Get the client name (e.g., /dev/ttys000) - needed for switch-client
	clientCmd := exec.Command("tmux", "display-message", "-p", "#{client_name}")
	clientOutput, err := clientCmd.Output()
	if err != nil {
		logging.Debug("Failed to get tmux client: %v", err)
		return nil
	}
	client := strings.TrimSpace(string(clientOutput))

	// Parse the target string
	// Format: "session_name:window_index.pane_index" (e.g., "main:0.2")
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 {
		logging.Debug("Unexpected tmux target format: %s", target)
		return nil
	}

	sessionName := parts[0]
	windowPane := parts[1]

	wpParts := strings.SplitN(windowPane, ".", 2)
	if len(wpParts) != 2 {
		logging.Debug("Unexpected tmux window.pane format: %s", windowPane)
		return nil
	}

	info := &TmuxPaneInfo{
		SessionName: sessionName,
		WindowIndex: wpParts[0],
		PaneIndex:   wpParts[1],
		Target:      target,
		Client:      client,
	}

	logging.Debug("Detected tmux pane: %s (client: %s)", target, client)
	return info
}

// BuildTmuxFocusCommand builds a shell command that will:
// 1. Switch to the correct tmux session/window/pane
// 2. Activate the terminal application
func BuildTmuxFocusCommand(pane *TmuxPaneInfo, terminalBundleID string) string {
	if pane == nil {
		return ""
	}

	// Find tmux binary path - terminal-notifier runs with minimal PATH
	tmuxPath := findTmuxPath()

	// Use switch-client with explicit client (-c) to switch the correct terminal
	// The client (e.g., /dev/ttys000) identifies which terminal to control
	// Finally, bring the terminal app to front with osascript
	terminalApp := bundleIDToAppName(terminalBundleID)
	cmd := tmuxPath + " switch-client -c '" + pane.Client + "' -t '" + pane.Target + "'; " +
		"osascript -e 'tell application \"" + terminalApp + "\" to activate'"

	logging.Debug("Built tmux focus command: %s", cmd)
	return cmd
}

// bundleIDToAppName converts a bundle ID to an application name for osascript
func bundleIDToAppName(bundleID string) string {
	mapping := map[string]string{
		"com.apple.Terminal":      "Terminal",
		"com.googlecode.iterm2":   "iTerm",
		"dev.warp.Warp-Stable":    "Warp",
		"net.kovidgoyal.kitty":    "kitty",
		"com.mitchellh.ghostty":   "Ghostty",
		"com.github.wez.wezterm":  "WezTerm",
		"org.alacritty":           "Alacritty",
		"co.zeit.hyper":           "Hyper",
		"com.microsoft.VSCode":    "Visual Studio Code",
	}

	if name, ok := mapping[bundleID]; ok {
		return name
	}
	return "Terminal" // fallback
}

// findTmuxPath returns the full path to tmux binary
func findTmuxPath() string {
	// Common paths for tmux
	paths := []string{
		"/usr/local/bin/tmux",  // Homebrew Intel
		"/opt/homebrew/bin/tmux", // Homebrew Apple Silicon
		"/usr/bin/tmux",        // System
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback - hope it's in PATH
	return "tmux"
}
