package profile

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nils-witt/wireless-terrain-planner/server/internal/geotiff"
	"github.com/nils-witt/wireless-terrain-planner/server/internal/utm"
)

const tileSize = 1000

// surface is linear in map coordinates. Bilinear interpolation reproduces a
// linear function exactly, so the profile's elevations are checkable in
// closed form.
func surface(x, y float64) float64 { return 100 + 0.03*(x-350000) + 0.02*(y-5650000) }

// writeTile writes an uncompressed float32 GeoTIFF for the tile whose
// south-west corner is (kx, ky) km. Pixel (row, col) holds surface() at the
// map coordinate its index maps to, unless skip reports it as nodata.
func writeTile(t testing.TB, dir string, kx, ky int, skip func(row, col int) bool) {
	t.Helper()
	originX, originY := float64(kx*1000), float64((ky+1)*1000)

	pix := make([]byte, tileSize*tileSize*4)
	for row := 0; row < tileSize; row++ {
		for col := 0; col < tileSize; col++ {
			v := float32(surface(originX+float64(col), originY-float64(row)))
			if skip != nil && skip(row, col) {
				v = -9999
			}
			binary.LittleEndian.PutUint32(pix[(row*tileSize+col)*4:], math.Float32bits(v))
		}
	}

	type tag struct {
		id, typ uint16
		count   uint32
		val     any // uint32 (inline) or []byte (out-of-line payload)
	}
	scale := f64s(1, 1, 0)
	tie := f64s(0, 0, 0, originX, originY, 0)
	nodata := []byte("-9999\x00")
	const ifdOff, nTags = 8, 13
	dataOff := uint32(ifdOff + 2 + nTags*12 + 4)
	tags := []tag{
		{256, 4, 1, uint32(tileSize)},
		{257, 4, 1, uint32(tileSize)},
		{258, 3, 1, uint32(32)},
		{259, 3, 1, uint32(1)},
		{262, 3, 1, uint32(1)},
		{273, 4, 1, dataOff}, // strip offset: pixel data first
		{277, 3, 1, uint32(1)},
		{278, 4, 1, uint32(tileSize)},
		{279, 4, 1, uint32(len(pix))},
		{339, 3, 1, uint32(3)},
		{33550, 12, 3, scale},
		{33922, 12, 6, tie},
		{42113, 2, uint32(len(nodata)), nodata},
	}

	var head, extra bytes.Buffer
	head.WriteString("II")
	binary.Write(&head, binary.LittleEndian, uint16(42))
	binary.Write(&head, binary.LittleEndian, uint32(ifdOff))
	binary.Write(&head, binary.LittleEndian, uint16(len(tags)))
	extraOff := dataOff + uint32(len(pix))
	for _, tg := range tags {
		binary.Write(&head, binary.LittleEndian, tg.id)
		binary.Write(&head, binary.LittleEndian, tg.typ)
		binary.Write(&head, binary.LittleEndian, tg.count)
		switch v := tg.val.(type) {
		case uint32:
			if tg.typ == 3 {
				binary.Write(&head, binary.LittleEndian, uint16(v))
				binary.Write(&head, binary.LittleEndian, uint16(0))
			} else {
				binary.Write(&head, binary.LittleEndian, v)
			}
		case []byte:
			binary.Write(&head, binary.LittleEndian, extraOff+uint32(extra.Len()))
			extra.Write(v)
		}
	}
	binary.Write(&head, binary.LittleEndian, uint32(0)) // no next IFD

	var file bytes.Buffer
	file.Write(head.Bytes())
	file.Write(pix)
	file.Write(extra.Bytes())
	name := filepath.Join(dir, tileKey{kx, ky}.filename())
	if err := os.WriteFile(name, file.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func f64s(v ...float64) []byte {
	b := make([]byte, 8*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint64(b[i*8:], math.Float64bits(x))
	}
	return b
}

func newSampler(dir string) *Sampler {
	return New(dir, 8, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// Line from (7.000, 51.000) to (7.030, 51.010): about 2.1 km east and 1.1 km
// north, so it crosses a few tiles.
var (
	lonA, latA = 7.000, 51.000
	lonB, latB = 7.030, 51.010
)

func tilesAround(t testing.TB, dir string) {
	ax, ay := utm.Forward(lonA, latA)
	bx, by := utm.Forward(lonB, latB)
	kx0, kx1 := int(math.Min(ax, bx)/1000), int(math.Max(ax, bx)/1000)
	ky0, ky1 := int(math.Min(ay, by)/1000), int(math.Max(ay, by)/1000)
	for kx := kx0; kx <= kx1; kx++ {
		for ky := ky0; ky <= ky1; ky++ {
			writeTile(t, dir, kx, ky, nil)
		}
	}
}

// checkProfile asserts the profile is ordered, starts at the first point, and
// that every elevation equals the analytic surface at the sample's location.
func checkProfile(t *testing.T, got [][2]float64, lon0, lat0, lon1, lat1 float64) {
	t.Helper()
	x0, y0 := utm.Forward(lon0, lat0)
	x1, y1 := utm.Forward(lon1, lat1)
	length := math.Hypot(x1-x0, y1-y0)

	if len(got) == 0 {
		t.Fatal("empty profile")
	}
	for i, p := range got {
		if i > 0 && p[0] <= got[i-1][0] {
			t.Fatalf("distance not strictly increasing at %d: %v then %v", i, got[i-1][0], p[0])
		}
		f := p[0] / length
		want := surface(x0+(x1-x0)*f, y0+(y1-y0)*f)
		if math.Abs(p[1]-want) > 1e-3 {
			t.Fatalf("sample %d at %.3f m: elevation %.6f, want %.6f", i, p[0], p[1], want)
		}
	}
	if got[0][0] != 0 {
		t.Errorf("first distance = %v, want 0", got[0][0])
	}
	if last := got[len(got)-1][0]; last > length+1e-6 || last < length-3 {
		t.Errorf("last distance = %v, want about %v", last, length)
	}
}

func TestProfileAcrossTiles(t *testing.T) {
	dir := t.TempDir()
	tilesAround(t, dir)
	s := newSampler(dir)

	ax, ay := utm.Forward(lonA, latA)
	bx, by := utm.Forward(lonB, latB)
	length := math.Hypot(bx-ax, by-ay)

	got, err := s.Profile(context.Background(), lonA, latA, lonB, latB, 1)
	if err != nil {
		t.Fatal(err)
	}
	checkProfile(t, got, lonA, latA, lonB, latB)

	// One sample per metre, minus the rare ones on a tile's last row/column
	// where bilinear interpolation has no neighbour.
	want := int(length) + 1
	if len(got) > want || len(got) < want-8 {
		t.Errorf("got %d samples, want about %d", len(got), want)
	}
}

// The original implementation only visited tiles between start and end in
// increasing x/y order; travelling west or south must work too.
func TestProfileAllDirections(t *testing.T) {
	dir := t.TempDir()
	tilesAround(t, dir)
	s := newSampler(dir)

	cases := map[string][4]float64{
		"west-south": {lonB, latB, lonA, latA},
		"east-south": {lonA, latB, lonB, latA},
		"west-north": {lonB, latA, lonA, latB},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := s.Profile(context.Background(), c[0], c[1], c[2], c[3], 1)
			if err != nil {
				t.Fatal(err)
			}
			checkProfile(t, got, c[0], c[1], c[2], c[3])
			if len(got) < 2000 {
				t.Errorf("only %d samples", len(got))
			}
		})
	}
}

func TestProfileMissingTileIsSkipped(t *testing.T) {
	dir := t.TempDir()
	tilesAround(t, dir)

	// Remove the middle tile the line passes through.
	ax, ay := utm.Forward(lonA, latA)
	bx, by := utm.Forward(lonB, latB)
	mx, my := (ax+bx)/2, (ay+by)/2
	mid := tileKey{int(mx / 1000), int(my / 1000)}
	if err := os.Remove(filepath.Join(dir, mid.filename())); err != nil {
		t.Fatal(err)
	}

	got, err := newSampler(dir).Profile(context.Background(), lonA, latA, lonB, latB, 1)
	if err != nil {
		t.Fatal(err)
	}
	checkProfile(t, got, lonA, latA, lonB, latB)
	length := math.Hypot(bx-ax, by-ay)
	if len(got) >= int(length) {
		t.Errorf("got %d samples, expected a gap for the missing tile (line is %.0f m)", len(got), length)
	}
	for _, p := range got {
		f := p[0] / length
		if (tileKey{int((ax + (bx-ax)*f) / 1000), int((ay + (by-ay)*f) / 1000)}) == mid {
			t.Fatalf("sample at %.1f m lies in the removed tile", p[0])
		}
	}
}

func TestProfileNodataIsSkipped(t *testing.T) {
	dir := t.TempDir()
	ax, ay := utm.Forward(lonA, latA)
	kx, ky := int(ax/1000), int(ay/1000)
	// Nodata across the whole tile: nothing to return, and no error.
	writeTile(t, dir, kx, ky, func(int, int) bool { return true })

	got, err := newSampler(dir).Profile(context.Background(), lonA, latA, lonA+0.0001, latA+0.0001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d samples from an all-nodata tile, want 0", len(got))
	}
}

func TestProfileNoTiles(t *testing.T) {
	got, err := newSampler(t.TempDir()).Profile(context.Background(), lonA, latA, lonB, latB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d samples with no tiles, want 0", len(got))
	}
}

func TestProfileIdenticalPoints(t *testing.T) {
	dir := t.TempDir()
	tilesAround(t, dir)
	got, err := newSampler(dir).Profile(context.Background(), lonA, latA, lonA, latA, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0][0] != 0 || got[1][0] != 0 || got[0][1] != got[1][1] {
		t.Errorf("got %v, want two samples at distance 0", got)
	}
}

func TestProfileHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newSampler(t.TempDir()).Profile(ctx, lonA, latA, lonB, latB, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestProfileConcurrent(t *testing.T) {
	dir := t.TempDir()
	tilesAround(t, dir)
	s := New(dir, 2, slog.New(slog.NewTextHandler(io.Discard, nil))) // small cache: force eviction

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.Profile(context.Background(), lonA, latA, lonB, latB, 1)
			if err != nil || len(got) < 2000 {
				t.Errorf("len = %d, err = %v", len(got), err)
			}
		}()
	}
	wg.Wait()
}

func TestTileCacheEvictsAndDoesNotCacheErrors(t *testing.T) {
	loads := map[string]int{}
	c := newTileCache("d", 2)
	c.load = func(path string) (*geotiff.Raster, error) {
		loads[path]++
		if filepath.Base(path) == (tileKey{9, 9}).filename() {
			return nil, os.ErrNotExist
		}
		return &geotiff.Raster{}, nil
	}
	get := func(x, y int) error { _, err := c.get(tileKey{x, y}); return err }

	get(1, 1)
	get(2, 2)
	get(1, 1) // hit, and now most recently used
	get(3, 3) // evicts (2,2)
	get(1, 1) // still cached
	get(2, 2) // reload
	if n := loads[filepath.Join("d", tileKey{1, 1}.filename())]; n != 1 {
		t.Errorf("tile 1,1 loaded %d times, want 1", n)
	}
	if n := loads[filepath.Join("d", tileKey{2, 2}.filename())]; n != 2 {
		t.Errorf("tile 2,2 loaded %d times, want 2 (evicted once)", n)
	}

	for i := 0; i < 3; i++ {
		if err := get(9, 9); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v", err)
		}
	}
	if n := loads[filepath.Join("d", tileKey{9, 9}.filename())]; n != 3 {
		t.Errorf("failing tile loaded %d times, want 3 (errors must not be cached)", n)
	}
}
