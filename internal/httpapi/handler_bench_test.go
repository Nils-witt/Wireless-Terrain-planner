package httpapi

import (
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkHandler(b *testing.B) {
	// A realistic response: about 2.4 km at 1 m spacing.
	pts := make([][2]float64, 2400)
	for i := range pts {
		pts[i] = [2]float64{float64(i) + 0.3141592653589793, 150 + 30*math.Sin(float64(i)/40)}
	}
	h := NewHandler(&fakeProfiler{profile: pts}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	b.ReportAllocs()
	for b.Loop() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", okQuery, nil))
		if rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}

type discardWriter struct{ h http.Header }

func (d discardWriter) Header() http.Header         { return d.h }
func (d discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d discardWriter) WriteHeader(int)             {}

// BenchmarkWriteProfile compares the response encoders on the same data
// without httptest's recorder buffering the body.
func BenchmarkWriteProfile(b *testing.B) {
	pts := make([][2]float64, 2400)
	for i := range pts {
		pts[i] = [2]float64{float64(i) + 0.3141592653589793, 150 + 30*math.Sin(float64(i)/40)}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := discardWriter{http.Header{}}

	b.Run("hand", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			writeProfile(w, pts, log)
		}
	})
	b.Run("encoding_json", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			writeJSON(w, http.StatusOK, pts, log)
		}
	})
}
