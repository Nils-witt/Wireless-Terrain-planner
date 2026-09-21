// Command server serves terrain elevation profiles computed from DGM1
// GeoTIFF tiles at /api, and the embedded frontend at /.
//
// Configuration (environment variables):
//
//	LISTEN_ADDR      address to listen on            (default ":8000")
//	TILE_DIR         directory holding the .tif tiles (default "/dgm1_tiff_kacheln")
//	TILE_CACHE_SIZE  decoded tiles kept in memory     (default 32, about 4 MB each)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nils-witt/wireless-terrain-planner/server/internal/httpapi"
	"github.com/nils-witt/wireless-terrain-planner/server/internal/profile"
	"github.com/nils-witt/wireless-terrain-planner/server/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	addr := env("LISTEN_ADDR", ":8000")
	dir := env("TILE_DIR", "dgm1_tiff_kacheln")
	cacheSize, err := strconv.Atoi(env("TILE_CACHE_SIZE", "32"))
	if err != nil || cacheSize < 1 {
		return errors.New("TILE_CACHE_SIZE must be a positive integer")
	}

	ui, err := web.Handler()
	if err != nil {
		log.Warn("serving the API only", "err", err)
		ui = http.NotFoundHandler()
	}
	mux := http.NewServeMux()
	mux.Handle("/api", httpapi.NewHandler(profile.New(dir, cacheSize, log), log))
	mux.Handle("/", ui)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", addr, "tile_dir", dir, "tile_cache_size", cacheSize)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
