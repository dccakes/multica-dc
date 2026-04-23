package cloudrunner

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultServerBaseURL      = "http://127.0.0.1:8080"
	defaultHeartbeatInterval  = 15 * time.Second
	defaultClaimInterval      = 3 * time.Second
	envServerBaseURL          = "MULTICA_SERVER_BASE_URL"
	envRuntimeID              = "MULTICA_CLOUDRUNNER_RUNTIME_ID"
	envRuntimeIDs             = "MULTICA_CLOUDRUNNER_RUNTIME_IDS"
	envAuthToken              = "MULTICA_CLOUDRUNNER_AUTH_TOKEN"
	envHeartbeatInterval      = "MULTICA_CLOUDRUNNER_HEARTBEAT_INTERVAL"
	envClaimInterval          = "MULTICA_CLOUDRUNNER_CLAIM_INTERVAL"
)

// Config controls cloudrunner lifecycle loops and daemon API connectivity.
type Config struct {
	ServerBaseURL      string
	RuntimeID          string
	RuntimeIDs         []string
	AuthToken          string
	HeartbeatInterval  time.Duration
	ClaimInterval      time.Duration
}

// LoadConfig reads cloudrunner config from environment variables.
func LoadConfig() (Config, error) {
	cfg := Config{
		ServerBaseURL: strings.TrimSpace(envOrDefault(envServerBaseURL, defaultServerBaseURL)),
		RuntimeID:     strings.TrimSpace(os.Getenv(envRuntimeID)),
		RuntimeIDs:    runtimeIDsFromCSV(os.Getenv(envRuntimeIDs)),
		AuthToken:     strings.TrimSpace(os.Getenv(envAuthToken)),
	}

	heartbeatInterval, err := durationFromEnv(envHeartbeatInterval, defaultHeartbeatInterval)
	if err != nil {
		return Config{}, err
	}
	claimInterval, err := durationFromEnv(envClaimInterval, defaultClaimInterval)
	if err != nil {
		return Config{}, err
	}

	cfg.HeartbeatInterval = heartbeatInterval
	cfg.ClaimInterval = claimInterval
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate ensures required fields exist and normalizes defaults.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ServerBaseURL) == "" {
		c.ServerBaseURL = defaultServerBaseURL
	}
	c.RuntimeIDs = runtimeIDsFromSlice(c.RuntimeIDs)
	if len(c.RuntimeIDs) == 0 {
		if runtimeID := strings.TrimSpace(c.RuntimeID); runtimeID != "" {
			c.RuntimeIDs = []string{runtimeID}
		}
	}
	if len(c.RuntimeIDs) == 0 {
		return fmt.Errorf("at least one runtime id is required (%s or %s)", envRuntimeID, envRuntimeIDs)
	}
	c.RuntimeID = c.RuntimeIDs[0]
	if strings.TrimSpace(c.AuthToken) == "" {
		return fmt.Errorf("auth token is required (%s)", envAuthToken)
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
	if c.ClaimInterval <= 0 {
		c.ClaimInterval = defaultClaimInterval
	}
	return nil
}

func runtimeIDsFromCSV(raw string) []string {
	return runtimeIDsFromSlice(strings.Split(raw, ","))
}

func runtimeIDsFromSlice(values []string) []string {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s: duration must be > 0", key)
	}
	return d, nil
}
