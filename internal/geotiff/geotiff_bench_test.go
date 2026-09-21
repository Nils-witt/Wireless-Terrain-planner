package geotiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"testing"
)

const benchTile = 1000 // a DGM1 tile is 1000x1000 at 1 m

// benchTerrain returns smooth, noisy terrain heights, row-major.
func benchTerrain() []float32 {
	a := make([]float32, benchTile*benchTile)
	for r := 0; r < benchTile; r++ {
		for c := 0; c < benchTile; c++ {
			z := 150 + 40*math.Sin(float64(c)/97) + 25*math.Cos(float64(r)/61) + 0.3*math.Sin(float64(r*c)/13)
			a[r*benchTile+c] = float32(math.Round(z*100) / 100)
		}
	}
	return a
}

// encodeStrips builds a little-endian float32 GeoTIFF with the floating point
// predictor (3) and the given compressor, rowsPerStrip rows per strip.
func encodeStrips(vals []float32, compression uint16, rowsPerStrip int, compress func([]byte) []byte) []byte {
	var strips [][]byte
	for y0 := 0; y0 < benchTile; y0 += rowsPerStrip {
		rows := min(rowsPerStrip, benchTile-y0)
		buf := make([]byte, rows*benchTile*4)
		for r := 0; r < rows; r++ {
			row := buf[r*benchTile*4 : (r+1)*benchTile*4]
			for c := 0; c < benchTile; c++ {
				bits := math.Float32bits(vals[(y0+r)*benchTile+c])
				// byte planes, most significant first
				row[0*benchTile+c] = byte(bits >> 24)
				row[1*benchTile+c] = byte(bits >> 16)
				row[2*benchTile+c] = byte(bits >> 8)
				row[3*benchTile+c] = byte(bits)
			}
			for i := len(row) - 1; i > 0; i-- {
				row[i] -= row[i-1]
			}
		}
		strips = append(strips, compress(buf))
	}

	le := binary.LittleEndian
	nTags := 13
	ifdOff := 8
	extraOff := ifdOff + 2 + nTags*12 + 4
	var extra bytes.Buffer
	putExtra := func(b []byte) uint32 {
		off := extraOff + extra.Len()
		extra.Write(b)
		return uint32(off)
	}
	offsets := make([]byte, 4*len(strips))
	counts := make([]byte, 4*len(strips))
	var data bytes.Buffer
	for i, s := range strips {
		le.PutUint32(counts[i*4:], uint32(len(s)))
	}
	scale := make([]byte, 24)
	for i, v := range []float64{1, 1, 0} {
		le.PutUint64(scale[i*8:], math.Float64bits(v))
	}
	tie := make([]byte, 48)
	for i, v := range []float64{0, 0, 0, 350000, 5651000, 0} {
		le.PutUint64(tie[i*8:], math.Float64bits(v))
	}
	nodata := []byte("-9999\x00")
	offOffsets := putExtra(offsets) // placeholder; rewritten after layout
	offCounts := putExtra(counts)
	offScale := putExtra(scale)
	offTie := putExtra(tie)
	offNodata := putExtra(nodata)
	dataBase := extraOff + extra.Len()
	pos := dataBase
	for i, s := range strips {
		le.PutUint32(offsets[i*4:], uint32(pos))
		data.Write(s)
		pos += len(s)
	}
	extraBytes := extra.Bytes()
	copy(extraBytes[int(offOffsets)-extraOff:], offsets)

	type tag struct {
		id, typ uint16
		count   uint32
		val     uint32
	}
	tags := []tag{
		{256, 4, 1, benchTile},
		{257, 4, 1, benchTile},
		{258, 3, 1, 32},
		{259, 3, 1, uint32(compression)},
		{273, 4, uint32(len(strips)), offOffsets},
		{277, 3, 1, 1},
		{278, 4, 1, uint32(rowsPerStrip)},
		{279, 4, uint32(len(strips)), offCounts},
		{317, 3, 1, 3},
		{339, 3, 1, 3},
		{33550, 12, 3, offScale},
		{33922, 12, 6, offTie},
		{42113, 2, uint32(len(nodata)), offNodata},
	}

	var out bytes.Buffer
	out.WriteString("II")
	binary.Write(&out, le, uint16(42))
	binary.Write(&out, le, uint32(ifdOff))
	binary.Write(&out, le, uint16(len(tags)))
	for _, tg := range tags {
		binary.Write(&out, le, tg.id)
		binary.Write(&out, le, tg.typ)
		binary.Write(&out, le, tg.count)
		if tg.typ == 3 && tg.count == 1 {
			binary.Write(&out, le, uint16(tg.val))
			binary.Write(&out, le, uint16(0))
		} else {
			binary.Write(&out, le, tg.val)
		}
	}
	binary.Write(&out, le, uint32(0))
	out.Write(extraBytes)
	out.Write(data.Bytes())
	return out.Bytes()
}

func deflate(b []byte) []byte {
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	zw.Write(b)
	zw.Close()
	return out.Bytes()
}

func identity(b []byte) []byte { return b }

func BenchmarkDecode(b *testing.B) {
	vals := benchTerrain()
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"deflate_pred3_strips16", encodeStrips(vals, compressionDeflate, 16, deflate)},
		{"none_pred3_strips16", encodeStrips(vals, compressionNone, 16, identity)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			r, err := Decode(tc.data)
			if err != nil {
				b.Fatal(err)
			}
			// Guard against a broken encoder silently benchmarking garbage.
			for _, i := range []int{0, 1234, 500_500, len(vals) - 1} {
				if r.Data[i] != vals[i] {
					b.Fatalf("pixel %d = %v, want %v", i, r.Data[i], vals[i])
				}
			}
			b.SetBytes(int64(len(vals) * 4))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Decode(tc.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
