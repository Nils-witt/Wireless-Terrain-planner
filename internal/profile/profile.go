// Package profile computes terrain elevation profiles between two points from
// DGM1 GeoTIFF tiles.
package profile

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"runtime"
	"sync"

	"github.com/nils-witt/wireless-terrain-planner/server/internal/geotiff"
	"github.com/nils-witt/wireless-terrain-planner/server/internal/utm"
)

// Sampler produces elevation profiles from a directory of DGM1 tiles named
// dgm1_32_<easting km>_<northing km>_1_nw_2021.tif.
type Sampler struct {
	tiles *tileCache
	log   *slog.Logger
}

// New returns a Sampler reading tiles from dir and keeping up to cacheSize
// decoded tiles in memory (each is about 4 MB).
func New(dir string, cacheSize int, log *slog.Logger) *Sampler {
	return &Sampler{tiles: newTileCache(dir, cacheSize), log: log}
}

// Profile samples the straight line from (lon0, lat0) to (lon1, lat1), given
// as WGS84 degrees, roughly every spacingM metres. Each result element is
// {distance from the start in metres, elevation in metres}, ordered by
// distance. Samples that fall in a missing tile, on nodata, or in the last
// row/column of a tile (where bilinear interpolation has no neighbour) are
// omitted.
func (s *Sampler) Profile(ctx context.Context, lon0, lat0, lon1, lat1, spacingM float64) ([][2]float64, error) {
	x0, y0 := utm.Forward(lon0, lat0)
	x1, y1 := utm.Forward(lon1, lat1)

	length := math.Hypot(x1-x0, y1-y0)
	n := 2
	if length > 0 {
		n = max(2, int(math.Floor(length/spacingM))+1)
	}

	var (
		// n is unbounded for far-apart points, so cap the up-front reservation.
		out     = make([][2]float64, 0, min(n, maxPrealloc))
		tiles   = s.loadTiles(ctx, x0, y0, x1, y1, n) // per-request; nil = unavailable
		lastKey tileKey
		last    *geotiff.Raster
		haveKey bool
	)
	for i := 0; i < n; i++ {
		if i&0xfff == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		t := float64(i) / float64(n-1)
		x := x0 + (x1-x0)*t
		y := y0 + (y1-y0)*t

		k := tileKey{int(math.Floor(x / 1000)), int(math.Floor(y / 1000))}
		if !haveKey || k != lastKey {
			r, seen := tiles[k]
			if !seen {
				r = s.loadTile(k)
				tiles[k] = r
			}
			lastKey, last, haveKey = k, r, true
		}
		if last == nil {
			continue
		}
		if z, ok := bilinear(last, x, y); ok {
			out = append(out, [2]float64{t * length, z})
		}
	}
	return out, nil
}

// maxPrealloc bounds the samples reserved up front (16 bytes each).
const maxPrealloc = 1 << 15

// prescanStride is how many samples apart loadTiles looks for the next tile.
// A tile is 1000 samples wide at 1 m spacing, so a line only escapes the scan
// by clipping a tile corner for a few metres; such a tile is loaded on demand.
const prescanStride = 16

// loadTiles loads the tiles the line is about to cross, concurrently, so a
// request over several uncached tiles pays for the slowest read and decode
// rather than their sum. It returns whichever tiles it loaded (nil for a
// missing one); Profile loads any tile the scan skipped itself.
func (s *Sampler) loadTiles(ctx context.Context, x0, y0, x1, y1 float64, n int) map[tileKey]*geotiff.Raster {
	tiles := map[tileKey]*geotiff.Raster{}
	var keys []tileKey
	for i := 0; ; i += prescanStride {
		i = min(i, n-1)
		t := float64(i) / float64(n-1)
		k := tileKey{int(math.Floor((x0 + (x1-x0)*t) / 1000)), int(math.Floor((y0 + (y1-y0)*t) / 1000))}
		if _, seen := tiles[k]; !seen {
			tiles[k] = nil
			keys = append(keys, k)
		}
		if i == n-1 {
			break
		}
	}
	if len(keys) < 2 {
		clear(tiles) // one tile gains nothing from a goroutine
		return tiles
	}

	loaded := make([]*geotiff.Raster, len(keys))
	sem := make(chan struct{}, min(len(keys), runtime.GOMAXPROCS(0)))
	var wg sync.WaitGroup
	for i, k := range keys {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			loaded[i] = s.loadTile(k)
		})
	}
	wg.Wait()
	for i, k := range keys {
		tiles[k] = loaded[i]
	}
	return tiles
}

func (s *Sampler) loadTile(k tileKey) *geotiff.Raster {
	r, err := s.tiles.get(k)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.log.Warn("tile not found", "tile", k.filename())
		} else {
			s.log.Error("reading tile", "tile", k.filename(), "err", err)
		}
		return nil
	}
	return r
}

// bilinear interpolates the raster at map coordinates (x, y). Pixel (row, col)
// is treated as sitting at fractional index (row, col) — the same convention
// rasterio's affine inverse gives — and the sample is rejected if it lacks
// any of its four neighbours or one of them is nodata.
func bilinear(r *geotiff.Raster, x, y float64) (float64, bool) {
	col := (x - r.OriginX) / r.PixelWidth
	row := (r.OriginY - y) / r.PixelHeight
	j0 := int(math.Floor(col))
	i0 := int(math.Floor(row))
	if i0 < 0 || j0 < 0 || i0+1 >= r.Height || j0+1 >= r.Width {
		return 0, false
	}
	di := row - float64(i0)
	dj := col - float64(j0)

	v00 := float64(r.At(i0, j0))
	v10 := float64(r.At(i0, j0+1))
	v01 := float64(r.At(i0+1, j0))
	v11 := float64(r.At(i0+1, j0+1))
	z := (1-dj)*(1-di)*v00 + dj*(1-di)*v10 + (1-dj)*di*v01 + dj*di*v11
	if math.IsNaN(z) || math.IsInf(z, 0) {
		return 0, false
	}
	return z, true
}
