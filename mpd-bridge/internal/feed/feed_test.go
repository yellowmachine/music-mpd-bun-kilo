package feed

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
)

type fakeMPD struct{ statusCalls int }

func (f *fakeMPD) Available() bool { return true }
func (f *fakeMPD) PlayerState(context.Context) (mpd.PlayerState, error) {
	f.statusCalls++
	return mpd.PlayerState{Status: mpd.Status{State: "play", Volume: 20, Random: true}}, nil
}
func (f *fakeMPD) Queue(context.Context) ([]mpd.QueueItem, error) { return []mpd.QueueItem{}, nil }

type fakeSnap struct{}

func (fakeSnap) Available() bool { return false }

type recorder map[string]string

func (r recorder) Publish(name string, v any) {
	b, _ := json.Marshal(v)
	r[name] = string(b)
}

func TestMPDChangedPayloads(t *testing.T) {
	m, rec := &fakeMPD{}, recorder{}
	f := New(m, fakeSnap{}, rec, slog.Default())

	f.MPDChanged([]string{"mixer", "options", "player", "playlist", "database", "stored_playlist", "mixer"})

	want := map[string]string{
		"player":          `{"status":{"state":"play","volume":20,"random":true,"repeat":false,"single":false,"consume":false,"playlistlength":0},"song":null}`,
		"mixer":           `{"volume":20}`,
		"options":         `{"consume":false,"random":true,"repeat":false,"single":false}`,
		"playlist":        `{"queue":[]}`,
		"database":        `{}`,
		"stored_playlist": `{}`,
	}
	for k, v := range want {
		if rec[k] != v {
			t.Errorf("%s = %s, want %s", k, rec[k], v)
		}
	}
	if m.statusCalls != 1 {
		t.Errorf("status fetched %d times per batch, want 1", m.statusCalls)
	}

	f.Connection()
	f.SnapClients(nil)
	if rec["connection"] != `{"mpd":true,"snap":false}` || rec["snap_clients"] != `{"clients":[]}` {
		t.Errorf("got %v", rec)
	}
}
