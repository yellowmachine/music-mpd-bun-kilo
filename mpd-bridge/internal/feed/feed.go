// Package feed turns MPD idle notifications and Snapcast changes into the
// events published on /events. Event names and payloads match what the app's
// own SSE endpoint already sends to browsers.
package feed

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
)

// Subsystems are the MPD idle subsystems the bridge watches.
var Subsystems = []string{"player", "mixer", "playlist", "options", "database", "stored_playlist"}

type MPD interface {
	Available() bool
	PlayerState(ctx context.Context) (mpd.PlayerState, error)
	Queue(ctx context.Context) ([]mpd.QueueItem, error)
}

type Snap interface {
	Available() bool
}

type Publisher interface {
	Publish(name string, v any)
}

type Feed struct {
	mpd  MPD
	snap Snap
	pub  Publisher
	log  *slog.Logger
}

func New(m MPD, s Snap, pub Publisher, log *slog.Logger) *Feed {
	return &Feed{mpd: m, snap: s, pub: pub, log: log.With("component", "feed")}
}

// MPDChanged publishes fresh state for each changed subsystem.
func (f *Feed) MPDChanged(subsystems []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Several subsystems need status; fetch it at most once per batch.
	var ps *mpd.PlayerState
	fetched := false
	state := func() *mpd.PlayerState {
		if !fetched {
			fetched = true
			if s, err := f.mpd.PlayerState(ctx); err != nil {
				f.logFetchError("status", err)
			} else {
				ps = &s
			}
		}
		return ps
	}

	slices.Sort(subsystems)
	for _, sub := range slices.Compact(subsystems) {
		switch sub {
		case "player":
			if s := state(); s != nil {
				f.pub.Publish("player", s)
			}
		case "mixer":
			if s := state(); s != nil {
				f.pub.Publish("mixer", map[string]int{"volume": s.Status.Volume})
			}
		case "options":
			if s := state(); s != nil {
				f.pub.Publish("options", map[string]bool{
					"random":  s.Status.Random,
					"repeat":  s.Status.Repeat,
					"single":  s.Status.Single,
					"consume": s.Status.Consume,
				})
			}
		case "playlist":
			q, err := f.mpd.Queue(ctx)
			if err != nil {
				f.logFetchError("queue", err)
				continue
			}
			f.pub.Publish("playlist", map[string]any{"queue": q})
		case "database", "stored_playlist":
			f.pub.Publish(sub, struct{}{})
		}
	}
}

// logFetchError stays quiet while MPD is simply down; the connection event
// already reports that.
func (f *Feed) logFetchError(what string, err error) {
	level := slog.LevelWarn
	if errors.Is(err, mpd.ErrUnavailable) {
		level = slog.LevelDebug
	}
	f.log.Log(context.Background(), level, "fetch "+what, "err", err)
}

// Resync republishes the full player state, for when changes may have been
// missed (MPD or the idle connection reconnected).
func (f *Feed) Resync() {
	f.MPDChanged([]string{"player", "mixer", "options", "playlist"})
}

// Connection publishes the availability of MPD and Snapserver.
func (f *Feed) Connection() {
	f.pub.Publish("connection", ConnectionState{MPD: f.mpd.Available(), Snap: f.snap.Available()})
}

// SnapClients publishes the Snapcast client list.
func (f *Feed) SnapClients(clients []snap.Client) {
	if clients == nil {
		clients = []snap.Client{}
	}
	f.pub.Publish("snap_clients", map[string]any{"clients": clients})
}

type ConnectionState struct {
	MPD  bool `json:"mpd"`
	Snap bool `json:"snap"`
}
