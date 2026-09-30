// Package httpapi exposes the bridge's closed set of HTTP endpoints.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/auth"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/events"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
)

// MPD is what the handlers need from the MPD client (mocked in tests).
type MPD interface {
	Available() bool
	PlayerState(ctx context.Context) (mpd.PlayerState, error)
	Transport(ctx context.Context, a mpd.Action) error
	PlayID(ctx context.Context, id int) error
	Seek(ctx context.Context, seconds float64) error
	SetVolume(ctx context.Context, v int) error
	SetOptions(ctx context.Context, o mpd.PlaybackOptions) error
	Queue(ctx context.Context) ([]mpd.QueueItem, error)
	AddToQueue(ctx context.Context, uris []string, replace, play bool) error
	ClearQueue(ctx context.Context) error
	DeleteID(ctx context.Context, id int) error
	ListInfo(ctx context.Context, dir string) (mpd.Listing, error)
	DBUpdateStamp(ctx context.Context) (int64, error)
	AllSongs(ctx context.Context) ([]mpd.Song, error)
	Update(ctx context.Context) (int, error)
	Playlists(ctx context.Context) ([]mpd.StoredPlaylist, error)
	PlaylistSongs(ctx context.Context, name string) ([]mpd.Song, error)
	PlaylistAdd(ctx context.Context, name, uri string) error
	PlaylistDelete(ctx context.Context, name string, pos int) error
	PlaylistMove(ctx context.Context, name string, from, to int) error
	PlaylistLoad(ctx context.Context, name string, replace, play bool) error
}

// Updater triggers an update of the bridge's own image (Watchtower).
type Updater interface {
	TriggerUpdate(ctx context.Context) error
}

// Snap is what the handlers need from the Snapcast client.
type Snap interface {
	Available() bool
	Clients() []snap.Client
	SetVolume(ctx context.Context, id string, percent *int, muted *bool) error
}

type Server struct {
	mpd       MPD
	snap      Snap
	updater   Updater // nil disables POST /admin/update
	broker    *events.Broker
	log       *slog.Logger
	lib       libraryCache
	heartbeat time.Duration
}

type Options struct {
	MPD  MPD
	Snap Snap
	// Updater, when set, enables POST /admin/update.
	Updater Updater
	Broker  *events.Broker
	Logger  *slog.Logger
	// Heartbeat is the SSE keep-alive interval; Cloudflare closes idle
	// connections after 100 s.
	Heartbeat time.Duration
}

func New(o Options) *Server {
	if o.Heartbeat == 0 {
		o.Heartbeat = 25 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Server{mpd: o.MPD, snap: o.Snap, updater: o.Updater, broker: o.Broker, log: o.Logger, heartbeat: o.Heartbeat}
}

// Handler returns the full handler chain: request logging, then auth, then
// routes. POST /admin/update takes updateToken instead of token, so the app's
// token can't trigger updates; it only exists when an Updater is set.
func (s *Server) Handler(token, updateToken string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /status", s.status)
	for _, a := range mpd.Actions {
		mux.HandleFunc("POST /player/"+string(a), s.transport(a))
	}
	mux.HandleFunc("POST /player/playid", s.playID)
	mux.HandleFunc("POST /player/seek", s.seek)
	mux.HandleFunc("PUT /volume", s.volume)
	mux.HandleFunc("PUT /options", s.options)

	mux.HandleFunc("GET /queue", s.queue)
	mux.HandleFunc("POST /queue", s.addToQueue)
	mux.HandleFunc("DELETE /queue", s.clearQueue)
	mux.HandleFunc("DELETE /queue/{id}", s.deleteFromQueue)

	mux.HandleFunc("GET /library/ls", s.listInfo)
	mux.HandleFunc("GET /library/all", s.allSongs)
	mux.HandleFunc("POST /library/update", s.update)

	mux.HandleFunc("GET /playlists", s.playlists)
	mux.HandleFunc("GET /playlists/{name}", s.playlistSongs)
	mux.HandleFunc("POST /playlists/{name}/songs", s.playlistAdd)
	mux.HandleFunc("DELETE /playlists/{name}/songs/{pos}", s.playlistDelete)
	mux.HandleFunc("POST /playlists/{name}/move", s.playlistMove)
	mux.HandleFunc("POST /playlists/{name}/load", s.playlistLoad)

	mux.HandleFunc("GET /snap/clients", s.snapClients)
	mux.HandleFunc("PUT /snap/clients/{id}/volume", s.snapVolume)

	mux.HandleFunc("GET /events", s.events)

	public := func(r *http.Request) bool { return r.Method == http.MethodGet && r.URL.Path == "/healthz" }
	root := http.NewServeMux()
	root.Handle("/", auth.Middleware(token, public, mux))
	if s.updater != nil {
		never := func(*http.Request) bool { return false }
		root.Handle("POST /admin/update", auth.Middleware(updateToken, never, http.HandlerFunc(s.selfUpdate)))
	}
	return s.logRequests(root)
}

// logRequests logs one line per request. It never logs headers, so the
// Authorization token can't end up in the logs.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		if r.URL.Path == "/healthz" {
			level = slog.LevelDebug
		}
		s.log.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach Flush and the deadline setters.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
