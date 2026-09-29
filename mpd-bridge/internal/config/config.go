// Package config loads the bridge configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
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

	token, file := getenv("API_TOKEN"), getenv("API_TOKEN_FILE")
	switch {
	case token != "" && file != "":
		errs = append(errs, errors.New("set only one of API_TOKEN and API_TOKEN_FILE"))
	case file != "":
		b, err := readFile(file)
		if err != nil {
			errs = append(errs, fmt.Errorf("API_TOKEN_FILE: %w", err))
		}
		token = string(b)
	}
	cfg.Token = strings.TrimSpace(token)
	switch {
	case cfg.Token == "":
		errs = append(errs, errors.New("API_TOKEN (or API_TOKEN_FILE) is required"))
	case len(cfg.Token) < MinTokenLength:
		errs = append(errs, fmt.Errorf("API_TOKEN must be at least %d characters (try: openssl rand -hex 32)", MinTokenLength))
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
