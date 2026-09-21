// Package httpapi is the HTTP interface of the elevation profile service.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
)

// spacingM is the distance between elevation samples along the profile.
const spacingM = 1

// Profiler computes an elevation profile between two WGS84 points.
type Profiler interface {
	Profile(ctx context.Context, lon0, lat0, lon1, lat1, spacingM float64) ([][2]float64, error)
}

// NewHandler returns the HTTP API:
//
//	GET /api?latitude1=..&longitude1=..&latitude2=..&longitude2=..
//
// It responds with a JSON array of [distance_m, elevation_m] pairs.
func NewHandler(p Profiler, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		lat1, err1 := coordinate(q, "latitude1", 90)
		lon1, err2 := coordinate(q, "longitude1", 180)
		lat2, err3 := coordinate(q, "latitude2", 90)
		lon2, err4 := coordinate(q, "longitude2", 180)
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()}, log)
			return
		}

		profile, err := p.Profile(r.Context(), lon1, lat1, lon2, lat2, spacingM)
		if err != nil {
			if r.Context().Err() == nil {
				log.Error("computing profile", "err", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"}, log)
			}
			return // client went away
		}
		writeProfile(w, profile, log)
	})
	return mux
}

// coordinate parses a required query parameter as a finite number within
// [-limit, limit].
func coordinate(q url.Values, name string, limit float64) (float64, error) {
	s := q.Get(name)
	if s == "" {
		return 0, fmt.Errorf("missing query parameter %q", name)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("query parameter %q is not a number: %q", name, s)
	}
	if math.Abs(v) > limit {
		return 0, fmt.Errorf("query parameter %q must be between -%g and %g", name, limit, limit)
	}
	return v, nil
}

func writeJSON(w http.ResponseWriter, status int, v any, log *slog.Logger) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Error("encoding response", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeBody(w, status, body)
}

// writeProfile responds with the profile as a JSON array of [distance,
// elevation] pairs; an empty profile is [] rather than null.
//
// The body is appended by hand instead of going through json.Marshal: at
// about 2400 pairs per request the reflection-based encoder cost ~260 µs and
// 27 allocations, several times the sampling itself.
func writeProfile(w http.ResponseWriter, profile [][2]float64, log *slog.Logger) {
	// Interpolated elevations and distances carry ~17 significant digits, so a
	// pair is about 40 bytes; sizing up front avoids append's doubling.
	body := make([]byte, 0, len(profile)*40+2)
	body = append(body, '[')
	for i, p := range profile {
		if !finite(p[0]) || !finite(p[1]) {
			// Let encoding/json report the unsupported value.
			writeJSON(w, http.StatusOK, profile, log)
			return
		}
		if i > 0 {
			body = append(body, ',')
		}
		body = append(body, '[')
		body = appendFloat(body, p[0])
		body = append(body, ',')
		body = appendFloat(body, p[1])
		body = append(body, ']')
	}
	body = append(body, ']')
	writeBody(w, http.StatusOK, body)
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	w.Write(body)
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// appendFloat formats f exactly as encoding/json does for a float64: shortest
// round-trip digits, in exponent form only for very small or very large
// magnitudes. f must be finite.
func appendFloat(b []byte, f float64) []byte {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format == 'e' {
		// Clean up e-09 to e-9, as encoding/json does.
		if n := len(b); n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return b
}
