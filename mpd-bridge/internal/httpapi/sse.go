package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/events"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/feed"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
)

// eventWriteTimeout bounds each write to a stream, so a stalled reader can't
// pin the handler now that the server-wide WriteTimeout is lifted.
const eventWriteTimeout = 10 * time.Second

type snapshot struct {
	Status      *mpd.Status          `json:"status"`
	Song        *mpd.Song            `json:"song"`
	Queue       []mpd.QueueItem      `json:"queue"`
	SnapClients []snap.Client        `json:"snap_clients"`
	Connection  feed.ConnectionState `json:"connection"`
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	ch, unsubscribe, err := s.broker.Subscribe()
	if err != nil {
		if errors.Is(err, events.ErrTooManySubscribers) {
			errorJSON(w, http.StatusServiceUnavailable, "too many event streams")
		} else {
			errorJSON(w, http.StatusServiceUnavailable, "shutting down")
		}
		return
	}
	defer unsubscribe()

	// The server's Read/WriteTimeout would cut the stream (an expired read
	// deadline cancels the request context), so lift both for this request.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Subscribed before taking the snapshot, so no change falls in between.
	if err := sendEvent(rc, w, "snapshot", s.snapshot(r.Context())); err != nil {
		return
	}

	t := time.NewTicker(s.heartbeat)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return // dropped as a slow subscriber, or shutting down
			}
			if err := writeFrame(rc, w, fmt.Sprintf("event: %s\ndata: %s\n\n", ev.Name, ev.Data)); err != nil {
				return
			}
		case <-t.C:
			if err := writeFrame(rc, w, ": ping\n\n"); err != nil {
				return
			}
		}
	}
}

func (s *Server) snapshot(ctx context.Context) snapshot {
	out := snapshot{
		Queue:       []mpd.QueueItem{},
		SnapClients: s.snap.Clients(),
		Connection:  feed.ConnectionState{MPD: s.mpd.Available(), Snap: s.snap.Available()},
	}
	if out.SnapClients == nil {
		out.SnapClients = []snap.Client{}
	}
	if ps, err := s.mpd.PlayerState(ctx); err == nil {
		out.Status, out.Song = &ps.Status, ps.Song
	}
	if q, err := s.mpd.Queue(ctx); err == nil {
		out.Queue = q
	}
	return out
}

func sendEvent(rc *http.ResponseController, w io.Writer, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFrame(rc, w, fmt.Sprintf("event: %s\ndata: %s\n\n", name, data))
}

func writeFrame(rc *http.ResponseController, w io.Writer, frame string) error {
	_ = rc.SetWriteDeadline(time.Now().Add(eventWriteTimeout))
	if _, err := io.WriteString(w, frame); err != nil {
		return err
	}
	return rc.Flush()
}
