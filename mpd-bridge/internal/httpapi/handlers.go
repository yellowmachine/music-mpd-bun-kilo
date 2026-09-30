package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/watchtower"
)

const (
	maxBodyBytes      = 64 << 10 // room for a POST /queue with maxQueueURIs stream URLs
	libraryAllTimeout = 60 * time.Second
)

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// fail maps an error to a response. Client-caused errors (4xx) carry MPD's
// message; anything else gets a generic message and is logged here instead.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ack *mpd.Error
	var rpc *snap.RPCError
	switch {
	case errors.Is(err, mpd.ErrUnavailable):
		errorJSON(w, http.StatusServiceUnavailable, "mpd unavailable")
	case errors.Is(err, snap.ErrUnavailable):
		errorJSON(w, http.StatusServiceUnavailable, "snap unavailable")
	case errors.Is(err, snap.ErrNotFound):
		errorJSON(w, http.StatusNotFound, "snap client not found")
	case errors.Is(err, mpd.ErrInvalidArg):
		errorJSON(w, http.StatusBadRequest, "invalid argument")
	case errors.As(err, &ack):
		switch ack.Code {
		case mpd.AckArg:
			errorJSON(w, http.StatusBadRequest, ack.Message)
		case mpd.AckNoExist:
			errorJSON(w, http.StatusNotFound, ack.Message)
		case mpd.AckExist, mpd.AckPlaylistMax, mpd.AckUpdateAlready:
			errorJSON(w, http.StatusConflict, ack.Message)
		default:
			s.log.Error("mpd error", "path", r.URL.Path, "err", err)
			errorJSON(w, http.StatusBadGateway, "mpd error")
		}
	case errors.As(err, &rpc):
		s.log.Error("snap error", "path", r.URL.Path, "err", err)
		errorJSON(w, http.StatusBadGateway, "snap error")
	case errors.Is(err, context.DeadlineExceeded):
		errorJSON(w, http.StatusGatewayTimeout, "timeout")
	case errors.Is(err, context.Canceled):
		// The client went away; nobody reads this.
		w.WriteHeader(499)
	default:
		s.log.Error("request failed", "path", r.URL.Path, "err", err)
		errorJSON(w, http.StatusInternalServerError, "internal error")
	}
}

// decode reads a single JSON object, rejecting unknown fields, trailing data
// and bodies over maxBodyBytes. It writes the error response itself.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("body must contain a single JSON object")
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			errorJSON(w, http.StatusRequestEntityTooLarge, "body too large")
		} else {
			errorJSON(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		}
		return false
	}
	return true
}

// decodeOptional is decode for endpoints whose body may be omitted entirely.
func decodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return decode(w, r, dst)
}

func (s *Server) done(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathInt(r *http.Request, name string) (int, bool) {
	n, err := strconv.Atoi(r.PathValue(name))
	return n, err == nil && n >= 0
}

func (s *Server) playlistName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if err := validPlaylistName(name); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return name, true
}

// --- health & player ---

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "mpd": s.mpd.Available(), "snap": s.snap.Available()})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ps, err := s.mpd.PlayerState(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) transport(a mpd.Action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.done(w, r, s.mpd.Transport(r.Context(), a))
	}
}

func (s *Server) playID(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID *int `json:"id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.ID == nil || *body.ID < 0 {
		errorJSON(w, http.StatusBadRequest, "id must be a non-negative integer")
		return
	}
	s.done(w, r, s.mpd.PlayID(r.Context(), *body.ID))
}

func (s *Server) seek(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Seconds *float64 `json:"seconds"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Seconds == nil || *body.Seconds < 0 || *body.Seconds > 24*3600 || math.IsNaN(*body.Seconds) {
		errorJSON(w, http.StatusBadRequest, "seconds must be between 0 and 86400")
		return
	}
	s.done(w, r, s.mpd.Seek(r.Context(), *body.Seconds))
}

func (s *Server) volume(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value *int `json:"value"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Value == nil || *body.Value < 0 || *body.Value > 100 {
		errorJSON(w, http.StatusBadRequest, "value must be an integer between 0 and 100")
		return
	}
	s.done(w, r, s.mpd.SetVolume(r.Context(), *body.Value))
}

func (s *Server) options(w http.ResponseWriter, r *http.Request) {
	var body mpd.PlaybackOptions
	if !decode(w, r, &body) {
		return
	}
	if body.Random == nil && body.Repeat == nil && body.Single == nil && body.Consume == nil {
		errorJSON(w, http.StatusBadRequest, "set at least one of random, repeat, single, consume")
		return
	}
	s.done(w, r, s.mpd.SetOptions(r.Context(), body))
}

// --- queue ---

func (s *Server) queue(w http.ResponseWriter, r *http.Request) {
	q, err := s.mpd.Queue(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) addToQueue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URIs    []string `json:"uris"`
		Replace bool     `json:"replace"`
		Play    bool     `json:"play"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.URIs) == 0 || len(body.URIs) > maxQueueURIs {
		errorJSON(w, http.StatusBadRequest, fmt.Sprintf("uris must contain 1 to %d items", maxQueueURIs))
		return
	}
	for i, u := range body.URIs {
		if err := validURI(u); err != nil {
			errorJSON(w, http.StatusBadRequest, fmt.Sprintf("uris[%d]: %v", i, err))
			return
		}
	}
	s.done(w, r, s.mpd.AddToQueue(r.Context(), body.URIs, body.Replace, body.Play))
}

func (s *Server) clearQueue(w http.ResponseWriter, r *http.Request) {
	s.done(w, r, s.mpd.ClearQueue(r.Context()))
}

func (s *Server) deleteFromQueue(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		errorJSON(w, http.StatusBadRequest, "id must be a non-negative integer")
		return
	}
	s.done(w, r, s.mpd.DeleteID(r.Context(), id))
}

// --- library ---

func (s *Server) listInfo(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if err := validLibraryPath(p); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	l, err := s.mpd.ListInfo(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// libraryCache holds the last /library/all body, keyed by MPD's db_update
// stamp, so repeated requests don't re-run listallinfo on the Pi.
type libraryCache struct {
	mu    sync.Mutex
	stamp int64
	raw   []byte
	gz    []byte
}

func (c *libraryCache) get(stamp int64) (raw, gz []byte, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.raw == nil || c.stamp != stamp {
		return nil, nil, false
	}
	return c.raw, c.gz, true
}

func (c *libraryCache) put(stamp int64, raw, gz []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stamp, c.raw, c.gz = stamp, raw, gz
}

func (s *Server) allSongs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), libraryAllTimeout)
	defer cancel()

	stamp, err := s.mpd.DBUpdateStamp(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	etag := fmt.Sprintf(`"db-%d"`, stamp)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Accept-Encoding")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	raw, gz, ok := s.lib.get(stamp)
	if !ok {
		songs, err := s.mpd.AllSongs(ctx)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		raw, err = json.Marshal(map[string]any{"db_update": stamp, "songs": songs})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		gz = buf.Bytes()
		s.lib.put(stamp, raw, gz)
	}

	w.Header().Set("Content-Type", "application/json")
	body := raw
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		w.Header().Set("Content-Encoding", "gzip")
		body = gz
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

func etagMatches(header, etag string) bool {
	for t := range strings.SplitSeq(header, ",") {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "W/"))
		if t == etag || t == "*" {
			return true
		}
	}
	return false
}

func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			return strings.ReplaceAll(strings.TrimSpace(params), " ", "") != "q=0"
		}
	}
	return false
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	job, err := s.mpd.Update(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"job": job})
}

// --- stored playlists ---

func (s *Server) playlists(w http.ResponseWriter, r *http.Request) {
	p, err := s.mpd.Playlists(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) playlistSongs(w http.ResponseWriter, r *http.Request) {
	name, ok := s.playlistName(w, r)
	if !ok {
		return
	}
	songs, err := s.mpd.PlaylistSongs(r.Context(), name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) playlistAdd(w http.ResponseWriter, r *http.Request) {
	name, ok := s.playlistName(w, r)
	if !ok {
		return
	}
	var body struct {
		URI string `json:"uri"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := validURI(body.URI); err != nil {
		errorJSON(w, http.StatusBadRequest, "uri: "+err.Error())
		return
	}
	s.done(w, r, s.mpd.PlaylistAdd(r.Context(), name, body.URI))
}

func (s *Server) playlistDelete(w http.ResponseWriter, r *http.Request) {
	name, ok := s.playlistName(w, r)
	if !ok {
		return
	}
	pos, ok := pathInt(r, "pos")
	if !ok {
		errorJSON(w, http.StatusBadRequest, "pos must be a non-negative integer")
		return
	}
	s.done(w, r, s.mpd.PlaylistDelete(r.Context(), name, pos))
}

func (s *Server) playlistMove(w http.ResponseWriter, r *http.Request) {
	name, ok := s.playlistName(w, r)
	if !ok {
		return
	}
	var body struct {
		From *int `json:"from"`
		To   *int `json:"to"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.From == nil || body.To == nil || *body.From < 0 || *body.To < 0 {
		errorJSON(w, http.StatusBadRequest, "from and to must be non-negative integers")
		return
	}
	s.done(w, r, s.mpd.PlaylistMove(r.Context(), name, *body.From, *body.To))
}

func (s *Server) playlistLoad(w http.ResponseWriter, r *http.Request) {
	name, ok := s.playlistName(w, r)
	if !ok {
		return
	}
	var body struct {
		Replace bool `json:"replace"`
		Play    bool `json:"play"`
	}
	if !decodeOptional(w, r, &body) {
		return
	}
	s.done(w, r, s.mpd.PlaylistLoad(r.Context(), name, body.Replace, body.Play))
}

// --- snapcast ---

func (s *Server) snapClients(w http.ResponseWriter, r *http.Request) {
	if !s.snap.Available() {
		s.fail(w, r, snap.ErrUnavailable)
		return
	}
	clients := s.snap.Clients()
	if clients == nil {
		clients = []snap.Client{}
	}
	writeJSON(w, http.StatusOK, clients)
}

func (s *Server) snapVolume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validSnapID(id); err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	var body struct {
		Percent *int  `json:"percent"`
		Muted   *bool `json:"muted"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Percent == nil && body.Muted == nil {
		errorJSON(w, http.StatusBadRequest, "set percent and/or muted")
		return
	}
	if body.Percent != nil && (*body.Percent < 0 || *body.Percent > 100) {
		errorJSON(w, http.StatusBadRequest, "percent must be an integer between 0 and 100")
		return
	}
	s.done(w, r, s.snap.SetVolume(r.Context(), id, body.Percent, body.Muted))
}

// selfUpdate asks Watchtower to pull new images of the bridge (and anything else
// it watches). It takes no input: the caller can only say "check now".
func (s *Server) selfUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	err := s.updater.TriggerUpdate(ctx)
	switch {
	case err == nil:
		s.log.Info("update triggered")
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "update started"})
	case errors.Is(err, watchtower.ErrBusy):
		errorJSON(w, http.StatusConflict, "an update is already running")
	case errors.Is(err, watchtower.ErrUnavailable):
		s.log.Error("watchtower unavailable", "err", err)
		errorJSON(w, http.StatusServiceUnavailable, "watchtower unavailable")
	default:
		s.log.Error("update failed", "err", err)
		errorJSON(w, http.StatusBadGateway, "watchtower error")
	}
}
