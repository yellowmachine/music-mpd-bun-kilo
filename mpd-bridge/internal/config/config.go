// Package config loads the bridge configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const MinTokenLength = 32

type Config struct {
	MPDAddr     string
	MPDPassword string
	SnapAddr    string // empty when SNAP_ENABLED=false
	ListenAddr  string
	Token       string
	LogLevel    slog.Level

	// Update webhook; empty UpdateToken disables POST /admin/update.
	UpdateToken     string
	WatchtowerURL   string
	WatchtowerToken string
}

// Load reads the configuration. getenv and readFile are injected for tests.
func Load(getenv func(string) string, readFile func(string) ([]byte, error)) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	var cfg Config
	var errs []error

	// secret reads KEY or KEY_FILE (Docker secrets), but not both.
	secret := func(key string) string {
		value, file := getenv(key), getenv(key+"_FILE")
		switch {
		case value != "" && file != "":
			errs = append(errs, fmt.Errorf("set only one of %s and %s_FILE", key, key))
		case file != "":
			b, err := readFile(file)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s_FILE: %w", key, err))
			}
			value = string(b)
		}
		return strings.TrimSpace(value)
	}

	mpdPort, err := port(get("MPD_PORT", "6600"))
	if err != nil {
		errs = append(errs, fmt.Errorf("MPD_PORT: %w", err))
	}
	cfg.MPDAddr = net.JoinHostPort(get("MPD_HOST", "127.0.0.1"), mpdPort)
	cfg.MPDPassword = getenv("MPD_PASSWORD")

	switch strings.ToLower(get("SNAP_ENABLED", "true")) {
	case "true", "1", "yes":
		snapPort, err := port(get("SNAP_PORT", "1705"))
		if err != nil {
			errs = append(errs, fmt.Errorf("SNAP_PORT: %w", err))
		}
		cfg.SnapAddr = net.JoinHostPort(get("SNAP_HOST", "127.0.0.1"), snapPort)
	case "false", "0", "no":
	default:
		errs = append(errs, errors.New("SNAP_ENABLED: must be true or false"))
	}

	cfg.ListenAddr = get("LISTEN_ADDR", "127.0.0.1:8787")
	if _, _, err := net.SplitHostPort(cfg.ListenAddr); err != nil {
		errs = append(errs, fmt.Errorf("LISTEN_ADDR: %w", err))
	}

	cfg.Token = secret("API_TOKEN")
	switch {
	case cfg.Token == "":
		errs = append(errs, errors.New("API_TOKEN (or API_TOKEN_FILE) is required"))
	case len(cfg.Token) < MinTokenLength:
		errs = append(errs, fmt.Errorf("API_TOKEN must be at least %d characters (try: openssl rand -hex 32)", MinTokenLength))
	}

	// The update webhook is optional; when UPDATE_TOKEN is set it needs
	// Watchtower. It has its own token so the app's can't trigger updates.
	cfg.UpdateToken = secret("UPDATE_TOKEN")
	if cfg.UpdateToken != "" {
		if len(cfg.UpdateToken) < MinTokenLength {
			errs = append(errs, fmt.Errorf("UPDATE_TOKEN must be at least %d characters (try: openssl rand -hex 32)", MinTokenLength))
		}
		if cfg.UpdateToken == cfg.Token {
			errs = append(errs, errors.New("UPDATE_TOKEN must differ from API_TOKEN"))
		}
		cfg.WatchtowerURL = get("WATCHTOWER_URL", "http://watchtower:8080")
		if u, err := url.Parse(cfg.WatchtowerURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, errors.New("WATCHTOWER_URL must be an http:// or https:// URL"))
		}
		if cfg.WatchtowerToken = secret("WATCHTOWER_TOKEN"); cfg.WatchtowerToken == "" {
			errs = append(errs, errors.New("WATCHTOWER_TOKEN (or WATCHTOWER_TOKEN_FILE) is required when UPDATE_TOKEN is set"))
		}
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	return cfg, errors.Join(errs...)
}

func port(s string) (string, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid port %q", s)
	}
	return s, nil
}
