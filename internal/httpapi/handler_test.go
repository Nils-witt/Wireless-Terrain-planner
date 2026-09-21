package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type fakeProfiler struct {
	profile                []([2]float64)
	err                    error
	lon0, lat0, lon1, lat1 float64
	spacing                float64
}

func (f *fakeProfiler) Profile(_ context.Context, lon0, lat0, lon1, lat1, spacing float64) ([][2]float64, error) {
	f.lon0, f.lat0, f.lon1, f.lat1, f.spacing = lon0, lat0, lon1, lat1, spacing
	return f.profile, f.err
}

func serve(p Profiler, method, target string) *httptest.ResponseRecorder {
	h := NewHandler(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

const okQuery = "/api?latitude1=52.0&longitude1=13.0&latitude2=52.1&longitude2=13.1"

func TestHandlerReturnsProfile(t *testing.T) {
	f := &fakeProfiler{profile: [][2]float64{{0, 100.5}, {1, 101}, {2.5, 99.25}}}
	rec := serve(f, "GET", okQuery)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if got, want := rec.Body.String(), `[[0,100.5],[1,101],[2.5,99.25]]`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(rec.Body.Len()) {
		t.Errorf("Content-Length = %q, want %d", cl, rec.Body.Len())
	}
	// Query params are lat/lon; the profiler takes lon/lat.
	if f.lon0 != 13.0 || f.lat0 != 52.0 || f.lon1 != 13.1 || f.lat1 != 52.1 || f.spacing != 1 {
		t.Errorf("profiler called with %+v", f)
	}
}

func TestHandlerEmptyProfileIsEmptyArray(t *testing.T) {
	rec := serve(&fakeProfiler{}, "GET", okQuery)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]" {
		t.Errorf("status %d body %q, want 200 []", rec.Code, rec.Body)
	}
}

func TestHandlerRejectsBadParameters(t *testing.T) {
	tests := map[string]string{
		"no params":       "/api",
		"missing lat2":    "/api?latitude1=52&longitude1=13&longitude2=13.1",
		"not a number":    "/api?latitude1=abc&longitude1=13&latitude2=52.1&longitude2=13.1",
		"NaN":             "/api?latitude1=NaN&longitude1=13&latitude2=52.1&longitude2=13.1",
		"infinite":        "/api?latitude1=Inf&longitude1=13&latitude2=52.1&longitude2=13.1",
		"latitude range":  "/api?latitude1=91&longitude1=13&latitude2=52.1&longitude2=13.1",
		"longitude range": "/api?latitude1=52&longitude1=13&latitude2=52.1&longitude2=-181",
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			f := &fakeProfiler{}
			rec := serve(f, "GET", target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), `"error"`) {
				t.Errorf("body %q is not a JSON error", rec.Body)
			}
			if f.spacing != 0 {
				t.Error("profiler was called for an invalid request")
			}
		})
	}
}

func TestHandlerProfilerFailure(t *testing.T) {
	rec := serve(&fakeProfiler{err: errors.New("disk on fire")}, "GET", okQuery)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "disk on fire") {
		t.Error("internal error detail leaked to the client")
	}
}

func TestHandlerRouting(t *testing.T) {
	if rec := serve(&fakeProfiler{}, "GET", "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want 404", rec.Code)
	}
	if rec := serve(&fakeProfiler{}, "POST", okQuery); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api = %d, want 405", rec.Code)
	}
	if rec := serve(&fakeProfiler{}, "HEAD", okQuery); rec.Code != http.StatusOK {
		t.Errorf("HEAD / = %d, want 200", rec.Code)
	}
}

// writeProfile hand-rolls the JSON; it must stay byte-identical to what
// encoding/json produces.
func TestWriteProfileMatchesEncodingJSON(t *testing.T) {
	profile := [][2]float64{
		{0, 0}, {1, -0.5}, {2.5, 99.25}, {3, 1e-6}, {4, 9.99e-7}, {5, 1e-9},
		{6, 1e20}, {7, 1e21}, {8, 1.5e300}, {9, -1e-7}, {10, 123456789.123456789},
		{0.1 + 0.2, 1.0 / 3.0}, {11, math.SmallestNonzeroFloat64}, {12, math.MaxFloat64},
	}
	for i := 0; i < 2000; i++ {
		profile = append(profile, [2]float64{float64(i) + 0.3141592653589793, 150 + 30*math.Sin(float64(i)/40)})
	}
	want, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	writeProfile(rec, profile, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("body differs from encoding/json:\n got %.200s\nwant %.200s", got, want)
	}
	if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(len(want)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(want))
	}
}

func TestWriteProfileNonFinite(t *testing.T) {
	rec := httptest.NewRecorder()
	writeProfile(rec, [][2]float64{{0, math.NaN()}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for an unencodable profile", rec.Code)
	}
}
