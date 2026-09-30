// Command mpd-bridge exposes a closed set of MPD and Snapcast actions over an
// authenticated HTTP API. Run "mpd-bridge healthcheck" to probe a running
// instance (used by the container HEALTHCHECK, since the image has no curl).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/config"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/events"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/feed"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/httpapi"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/watchtower"
)

// maxEventStreams caps concurrent /events connections; normally only the
// app's backend holds one.
const maxEventStreams = 16

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load(os.Getenv, os.ReadFile)
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		return 1
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	broker := events.NewBroker(maxEventStreams, log)

	// The clients report changes through callbacks into the feed, which in
	// turn reads from the clients, so the feed is assigned after both exist.
	// Nothing calls back before the goroutines below start.
	var fd *feed.Feed
	mpdClient := mpd.NewClient(mpd.Config{
		Addr:     cfg.MPDAddr,
		Password: cfg.MPDPassword,
		Logger:   log,
		OnAvailability: func(up bool) {
			fd.Connection()
			if up {
				go fd.Resync()
			}
		},
	})
	snapClient := snap.New(snap.Config{
		Addr:           cfg.SnapAddr,
		Logger:         log,
		OnChange:       func(c []snap.Client) { fd.SnapClients(c) },
		OnAvailability: func(bool) { fd.Connection() },
	})
	fd = feed.New(mpdClient, snapClient, broker, log)

	var wg sync.WaitGroup
	wg.Go(func() { mpdClient.Run(ctx) })
	wg.Go(func() { mpdClient.Watch(ctx, feed.Subsystems, fd.Resync, fd.MPDChanged) })
	wg.Go(func() { snapClient.Run(ctx) })

	var updater httpapi.Updater
	if cfg.UpdateToken != "" {
		updater = watchtower.New(cfg.WatchtowerURL, cfg.WatchtowerToken)
	}

	api := httpapi.New(httpapi.Options{MPD: mpdClient, Snap: snapClient, Updater: updater, Broker: broker, Logger: log})
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.Handler(cfg.Token, cfg.UpdateToken),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      75 * time.Second, // > the 60 s /library/all budget; /events lifts it
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.ListenAddr, "mpd", cfg.MPDAddr, "snap", cfg.SnapAddr, "update_webhook", updater != nil)

	code := 0
	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-serveErr:
		log.Error("server failed", "err", err)
		code = 1
		stop()
	}

	// Close the event streams first: Shutdown waits for active requests.
	broker.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("shutdown", "err", err)
		code = 1
	}
	wg.Wait()
	return code
}

// healthcheck probes /healthz on LISTEN_ADDR and exits 0 on 200. The bridge
// is healthy even while MPD is down; that is reported in the body instead.
func healthcheck() int {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
