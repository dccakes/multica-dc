package cloudrunner

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_UsesDefaultsAndValidatesRequiredFields(t *testing.T) {
	t.Setenv(envServerBaseURL, "")
	t.Setenv(envRuntimeID, "runtime-123")
	t.Setenv(envRuntimeIDs, "")
	t.Setenv(envAuthToken, "token-123")
	t.Setenv(envHeartbeatInterval, "")
	t.Setenv(envClaimInterval, "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.ServerBaseURL != defaultServerBaseURL {
		t.Fatalf("ServerBaseURL = %q, want %q", cfg.ServerBaseURL, defaultServerBaseURL)
	}
	if cfg.HeartbeatInterval != defaultHeartbeatInterval {
		t.Fatalf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, defaultHeartbeatInterval)
	}
	if cfg.ClaimInterval != defaultClaimInterval {
		t.Fatalf("ClaimInterval = %v, want %v", cfg.ClaimInterval, defaultClaimInterval)
	}
	if len(cfg.RuntimeIDs) != 1 || cfg.RuntimeIDs[0] != "runtime-123" {
		t.Fatalf("RuntimeIDs = %#v, want [runtime-123]", cfg.RuntimeIDs)
	}
}

func TestLoadConfig_RuntimeIDs_Missing(t *testing.T) {
	t.Setenv(envServerBaseURL, "http://localhost:8080")
	t.Setenv(envRuntimeID, "")
	t.Setenv(envRuntimeIDs, "")
	t.Setenv(envAuthToken, "token-123")

	_, err := LoadConfig()
	if err == nil || (!strings.Contains(err.Error(), envRuntimeID) && !strings.Contains(err.Error(), envRuntimeIDs)) {
		t.Fatalf("LoadConfig() error = %v, want missing runtime id error", err)
	}
}

func TestLoadConfig_MissingRequiredAuthToken(t *testing.T) {
	t.Setenv(envServerBaseURL, "http://localhost:8080")
	t.Setenv(envRuntimeID, "runtime-123")
	t.Setenv(envRuntimeIDs, "")
	t.Setenv(envAuthToken, "")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), envAuthToken) {
		t.Fatalf("LoadConfig() error = %v, want missing auth token error", err)
	}
}

func TestLoadConfig_RuntimeIDsCommaSeparatedPreferredAndNormalized(t *testing.T) {
	t.Setenv(envRuntimeID, "legacy-runtime")
	t.Setenv(envRuntimeIDs, "  rt-1, rt-2,, rt-3  ")
	t.Setenv(envAuthToken, "token-123")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := []string{"rt-1", "rt-2", "rt-3"}
	if len(cfg.RuntimeIDs) != len(want) {
		t.Fatalf("RuntimeIDs len = %d, want %d (%#v)", len(cfg.RuntimeIDs), len(want), cfg.RuntimeIDs)
	}
	for i := range want {
		if cfg.RuntimeIDs[i] != want[i] {
			t.Fatalf("RuntimeIDs[%d] = %q, want %q", i, cfg.RuntimeIDs[i], want[i])
		}
	}
	if cfg.RuntimeID != "rt-1" {
		t.Fatalf("RuntimeID = %q, want rt-1", cfg.RuntimeID)
	}
}

func TestLoadConfig_InvalidDuration(t *testing.T) {
	t.Setenv(envRuntimeID, "runtime-123")
	t.Setenv(envRuntimeIDs, "")
	t.Setenv(envAuthToken, "token-123")
	t.Setenv(envHeartbeatInterval, "not-a-duration")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), envHeartbeatInterval) {
		t.Fatalf("LoadConfig() error = %v, want duration parse error", err)
	}
}

func TestDurationFromEnv_PositiveDuration(t *testing.T) {
	const key = "MULTICA_TEST_DURATION"
	defer os.Unsetenv(key)
	if err := os.Setenv(key, "2s"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	got, err := durationFromEnv(key, time.Second)
	if err != nil {
		t.Fatalf("durationFromEnv() error = %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("durationFromEnv() = %v, want 2s", got)
	}
}
