package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ServiceConfig holds configuration for OS service installation.
type ServiceConfig struct {
	BinaryPath string // Full path to the overhuman binary
	DataDir    string // Data directory (~/.overhuman)
	APIAddr    string // API listen address
	AgentName  string // Agent name
}

// InstallResult contains the result of service installation.
type InstallResult struct {
	ServiceFile  string // Path to the installed service file
	Platform     string // "launchd" or "systemd"
	Instructions string // Human-readable instructions
}

// Install installs the overhuman daemon as an OS service.
func Install(cfg ServiceConfig) (*InstallResult, error) {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(cfg)
	case "linux":
		return installSystemd(cfg)
	default:
		return nil, fmt.Errorf("unsupported platform: %s (use macOS or Linux)", runtime.GOOS)
	}
}

// Uninstall removes the OS service.
func Uninstall() (*InstallResult, error) {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// --- launchd (macOS) ---

const launchdLabel = "com.overhuman.agent"

func launchdPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

// GenerateLaunchdPlist generates the plist XML for macOS launchd.
func GenerateLaunchdPlist(cfg ServiceConfig) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + launchdLabel + `</string>
  <key>ProgramArguments</key>
  <array>
    <string>` + cfg.BinaryPath + `</string>
    <string>start</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>OVERHUMAN_DATA</key>
    <string>` + cfg.DataDir + `</string>
    <key>OVERHUMAN_API_ADDR</key>
    <string>` + cfg.APIAddr + `</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>` + filepath.Join(cfg.DataDir, "logs", "overhuman.log") + `</string>
  <key>StandardErrorPath</key>
  <string>` + filepath.Join(cfg.DataDir, "logs", "overhuman.err") + `</string>
  <key>WorkingDirectory</key>
  <string>` + cfg.DataDir + `</string>
</dict>
</plist>
`)
	return sb.String()
}

func installLaunchd(cfg ServiceConfig) (*InstallResult, error) {
	plistPath := launchdPlistPath()

	// Ensure LaunchAgents directory exists.
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return nil, fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	// Ensure logs directory exists.
	logsDir := filepath.Join(cfg.DataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	plist := GenerateLaunchdPlist(cfg)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return nil, fmt.Errorf("write plist: %w", err)
	}

	return &InstallResult{
		ServiceFile: plistPath,
		Platform:    "launchd",
		Instructions: fmt.Sprintf(`Service installed successfully!

  Service file: %s

  Start now:    launchctl load %s
  Stop:         launchctl unload %s
  Logs:         tail -f %s/logs/overhuman.log

The daemon will start automatically on login.`, plistPath, plistPath, plistPath, cfg.DataDir),
	}, nil
}

func uninstallLaunchd() (*InstallResult, error) {
	plistPath := launchdPlistPath()
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("service not installed (no plist at %s)", plistPath)
	}

	if err := os.Remove(plistPath); err != nil {
		return nil, fmt.Errorf("remove plist: %w", err)
	}

	return &InstallResult{
		ServiceFile: plistPath,
		Platform:    "launchd",
		Instructions: fmt.Sprintf(`Service uninstalled.

  Removed: %s

  If the service was running, also run:
    launchctl unload %s`, plistPath, plistPath),
	}, nil
}

// --- systemd (Linux) ---

const systemdServiceName = "overhuman.service"

func systemdServicePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", systemdServiceName)
}

// GenerateSystemdUnit generates the systemd unit file content.
func GenerateSystemdUnit(cfg ServiceConfig) string {
	var sb strings.Builder
	sb.WriteString(`[Unit]
Description=Overhuman AI Assistant Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=` + cfg.BinaryPath + ` start
Environment=OVERHUMAN_DATA=` + cfg.DataDir + `
Environment=OVERHUMAN_API_ADDR=` + cfg.APIAddr + `
WorkingDirectory=` + cfg.DataDir + `
Restart=on-failure
RestartSec=5
StandardOutput=append:` + filepath.Join(cfg.DataDir, "logs", "overhuman.log") + `
StandardError=append:` + filepath.Join(cfg.DataDir, "logs", "overhuman.err") + `

[Install]
WantedBy=default.target
`)
	return sb.String()
}

func installSystemd(cfg ServiceConfig) (*InstallResult, error) {
	unitPath := systemdServicePath()

	// Ensure systemd user directory exists.
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return nil, fmt.Errorf("create systemd dir: %w", err)
	}

	// Ensure logs directory exists.
	logsDir := filepath.Join(cfg.DataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	unit := GenerateSystemdUnit(cfg)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return nil, fmt.Errorf("write unit file: %w", err)
	}

	return &InstallResult{
		ServiceFile: unitPath,
		Platform:    "systemd",
		Instructions: fmt.Sprintf(`Service installed successfully!

  Unit file: %s

  Enable:  systemctl --user daemon-reload && systemctl --user enable overhuman
  Start:   systemctl --user start overhuman
  Stop:    systemctl --user stop overhuman
  Status:  systemctl --user status overhuman
  Logs:    journalctl --user -u overhuman -f

  For auto-start after boot (without login):
    sudo loginctl enable-linger $USER`, unitPath),
	}, nil
}

func uninstallSystemd() (*InstallResult, error) {
	unitPath := systemdServicePath()
	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("service not installed (no unit at %s)", unitPath)
	}

	if err := os.Remove(unitPath); err != nil {
		return nil, fmt.Errorf("remove unit file: %w", err)
	}

	return &InstallResult{
		ServiceFile: unitPath,
		Platform:    "systemd",
		Instructions: fmt.Sprintf(`Service uninstalled.

  Removed: %s

  Also run:
    systemctl --user daemon-reload
    systemctl --user disable overhuman`, unitPath),
	}, nil
}
