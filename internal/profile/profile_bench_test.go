package profile

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func BenchmarkProfile(b *testing.B) {
	dir := b.TempDir()
	tilesAround(b, dir)
	s := New(dir, 32, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	// Warm the tile cache: this benchmark measures sampling, not tile decoding.
	got, err := s.Profile(ctx, lonA, latA, lonB, latB, 1)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(len(got)), "samples")

	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Profile(ctx, lonA, latA, lonB, latB, 1); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProfileCold measures a request that finds every tile missing from
// the cache: file read plus decode for each tile the line crosses.
func BenchmarkProfileCold(b *testing.B) {
	dir := b.TempDir()
	tilesAround(b, dir)
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		s := New(dir, 32, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if _, err := s.Profile(ctx, lonA, latA, lonB, latB, 1); err != nil {
			b.Fatal(err)
		}
	}
}
