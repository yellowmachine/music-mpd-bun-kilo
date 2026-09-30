package config

import (
	"errors"
	"strings"
	"testing"
)

const token = "0123456789abcdef0123456789abcdef"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func noFile(string) ([]byte, error) { return nil, errors.New("no file") }

func TestDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"API_TOKEN": token}), noFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MPDAddr != "127.0.0.1:6600" || cfg.SnapAddr != "127.0.0.1:1705" || cfg.ListenAddr != "127.0.0.1:8787" {
		t.Errorf("got %+v", cfg)
	}
}

func TestTokenRequiredAndLongEnough(t *testing.T) {
	if _, err := Load(env(nil), noFile); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("missing token: got %v", err)
	}
	if _, err := Load(env(map[string]string{"API_TOKEN": "short"}), noFile); err == nil {
		t.Error("short token accepted")
	}
	both := map[string]string{"API_TOKEN": token, "API_TOKEN_FILE": "/run/secrets/t"}
	if _, err := Load(env(both), noFile); err == nil {
		t.Error("both token sources accepted")
	}
}

func TestTokenFileIsTrimmed(t *testing.T) {
	read := func(string) ([]byte, error) { return []byte(token + "\n"), nil }
	cfg, err := Load(env(map[string]string{"API_TOKEN_FILE": "/run/secrets/t"}), read)
	if err != nil || cfg.Token != token {
		t.Errorf("got %q, %v", cfg.Token, err)
	}
}

func TestSnapDisabledAndBadPort(t *testing.T) {
	cfg, err := Load(env(map[string]string{"API_TOKEN": token, "SNAP_ENABLED": "false"}), noFile)
	if err != nil || cfg.SnapAddr != "" {
		t.Errorf("got %q, %v", cfg.SnapAddr, err)
	}
	if _, err := Load(env(map[string]string{"API_TOKEN": token, "MPD_PORT": "99999"}), noFile); err == nil {
		t.Error("bad port accepted")
	}
}

func TestUpdateWebhook(t *testing.T) {
	cfg, err := Load(env(map[string]string{"API_TOKEN": token}), noFile)
	if err != nil || cfg.UpdateToken != "" {
		t.Errorf("disabled by default: got %+v, %v", cfg, err)
	}

	m := map[string]string{"API_TOKEN": token, "UPDATE_TOKEN": strings.Repeat("u", 32)}
	if _, err := Load(env(m), noFile); err == nil || !strings.Contains(err.Error(), "WATCHTOWER_TOKEN") {
		t.Errorf("missing watchtower token: got %v", err)
	}
	m["WATCHTOWER_TOKEN"] = "wt"
	cfg, err = Load(env(m), noFile)
	if err != nil || cfg.WatchtowerURL != "http://watchtower:8080" || cfg.WatchtowerToken != "wt" {
		t.Errorf("got %+v, %v", cfg, err)
	}

	m["UPDATE_TOKEN"] = token
	if _, err := Load(env(m), noFile); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Errorf("same token as API_TOKEN: got %v", err)
	}
	m["UPDATE_TOKEN"] = "short"
	if _, err := Load(env(m), noFile); err == nil {
		t.Error("short update token accepted")
	}
}
