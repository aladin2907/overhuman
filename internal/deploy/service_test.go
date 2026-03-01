package deploy

import (
	"strings"
	"testing"
)

func testServiceConfig() ServiceConfig {
	return ServiceConfig{
		BinaryPath: "/usr/local/bin/overhuman",
		DataDir:    "/home/user/.overhuman",
		APIAddr:    "127.0.0.1:9090",
		AgentName:  "my-agent",
	}
}

func TestGenerateLaunchdPlist_ContainsBinary(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, cfg.BinaryPath) {
		t.Fatalf("plist does not contain binary path %q", cfg.BinaryPath)
	}
}

func TestGenerateLaunchdPlist_ContainsDataDir(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, cfg.DataDir) {
		t.Fatalf("plist does not contain data dir %q", cfg.DataDir)
	}
}

func TestGenerateLaunchdPlist_ContainsLabel(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, "com.overhuman.agent") {
		t.Fatal("plist does not contain label com.overhuman.agent")
	}
}

func TestGenerateLaunchdPlist_ContainsRunAtLoad(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, "<key>RunAtLoad</key>") {
		t.Fatal("plist does not contain RunAtLoad key")
	}
	if !strings.Contains(plist, "<true/>") {
		t.Fatal("plist does not contain <true/> for RunAtLoad")
	}
}

func TestGenerateLaunchdPlist_ContainsKeepAlive(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, "<key>KeepAlive</key>") {
		t.Fatal("plist does not contain KeepAlive key")
	}
}

func TestGenerateLaunchdPlist_ContainsLogPaths(t *testing.T) {
	cfg := testServiceConfig()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, "StandardOutPath") {
		t.Fatal("plist does not contain StandardOutPath")
	}
	if !strings.Contains(plist, "StandardErrorPath") {
		t.Fatal("plist does not contain StandardErrorPath")
	}
	if !strings.Contains(plist, "overhuman.log") {
		t.Fatal("plist does not contain overhuman.log")
	}
	if !strings.Contains(plist, "overhuman.err") {
		t.Fatal("plist does not contain overhuman.err")
	}
}

func TestGenerateSystemdUnit_ContainsBinary(t *testing.T) {
	cfg := testServiceConfig()
	unit := GenerateSystemdUnit(cfg)

	if !strings.Contains(unit, "ExecStart="+cfg.BinaryPath) {
		t.Fatalf("unit does not contain ExecStart with binary path %q", cfg.BinaryPath)
	}
}

func TestGenerateSystemdUnit_ContainsDataDir(t *testing.T) {
	cfg := testServiceConfig()
	unit := GenerateSystemdUnit(cfg)

	if !strings.Contains(unit, "WorkingDirectory="+cfg.DataDir) {
		t.Fatalf("unit does not contain WorkingDirectory=%s", cfg.DataDir)
	}
}

func TestGenerateSystemdUnit_ContainsRestart(t *testing.T) {
	cfg := testServiceConfig()
	unit := GenerateSystemdUnit(cfg)

	if !strings.Contains(unit, "Restart=on-failure") {
		t.Fatal("unit does not contain Restart=on-failure")
	}
}

func TestGenerateSystemdUnit_ContainsAfterNetwork(t *testing.T) {
	cfg := testServiceConfig()
	unit := GenerateSystemdUnit(cfg)

	if !strings.Contains(unit, "After=network-online.target") {
		t.Fatal("unit does not contain After=network-online.target")
	}
}

func TestGenerateSystemdUnit_ContainsEnvironment(t *testing.T) {
	cfg := testServiceConfig()
	unit := GenerateSystemdUnit(cfg)

	if !strings.Contains(unit, "Environment=OVERHUMAN_DATA="+cfg.DataDir) {
		t.Fatal("unit does not contain OVERHUMAN_DATA environment variable")
	}
	if !strings.Contains(unit, "Environment=OVERHUMAN_API_ADDR="+cfg.APIAddr) {
		t.Fatal("unit does not contain OVERHUMAN_API_ADDR environment variable")
	}
}

func TestServiceConfig_Roundtrip(t *testing.T) {
	cfg := testServiceConfig()

	plist := GenerateLaunchdPlist(cfg)
	if plist == "" {
		t.Fatal("GenerateLaunchdPlist returned empty string")
	}

	unit := GenerateSystemdUnit(cfg)
	if unit == "" {
		t.Fatal("GenerateSystemdUnit returned empty string")
	}

	// Both should be substantial documents.
	if len(plist) < 100 {
		t.Fatalf("plist too short: %d bytes", len(plist))
	}
	if len(unit) < 100 {
		t.Fatalf("unit too short: %d bytes", len(unit))
	}
}
