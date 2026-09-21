// Package geotiff reads single-band elevation rasters stored as GeoTIFF.
//
// It is a deliberately small reader: classic (non-Big) TIFF, one sample per
// pixel, strips or tiles, uncompressed / LZW / Deflate compression, and the
// horizontal-differencing and floating-point predictors. Georeferencing comes
// from the ModelPixelScale and ModelTiepoint tags, and the nodata value from
// GDAL_NODATA. That covers the DGM1 tiles this service consumes without
// needing GDAL.
package geotiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"golang.org/x/image/tiff/lzw"
)

// TIFF tag IDs used by the reader.
const (
	tagImageWidth      = 256
	tagImageLength     = 257
	tagBitsPerSample   = 258
	tagCompression     = 259
	tagStripOffsets    = 273
	tagSamplesPerPixel = 277
	tagRowsPerStrip    = 278
	tagStripByteCounts = 279
	tagPlanarConfig    = 284
	tagPredictor       = 317
	tagTileWidth       = 322
	tagTileLength      = 323
	tagTileOffsets     = 324
	tagTileByteCounts  = 325
	tagSampleFormat    = 339
	tagPixelScale      = 33550
	tagTiepoint        = 33922
	tagGDALNoData      = 42113
)

const (
	compressionNone        = 1
	compressionLZW         = 5
	compressionDeflate     = 8
	compressionDeflateOld  = 32946
	predictorNone          = 1
	predictorHorizontal    = 2
	predictorFloatingPoint = 3
)

// Raster is a decoded single-band raster. Pixel (row, col) covers the map
// area starting at (OriginX + col*PixelWidth, OriginY - row*PixelHeight)
// and extending right and down (PixelHeight is positive for north-up
// rasters).
type Raster struct {
	Width, Height int
	// Data holds Width*Height values in row-major order. Nodata pixels are NaN.
	Data []float32

	OriginX, OriginY        float64
	PixelWidth, PixelHeight float64
}

// At returns the value of the pixel at (row, col). The caller must ensure the
// indices are within bounds.
func (r *Raster) At(row, col int) float32 { return r.Data[row*r.Width+col] }

// Open reads and decodes the GeoTIFF at path.
func Open(path string) (*Raster, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r, err := Decode(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// ifd holds the decoded tags of the first image file directory.
type ifd struct {
	order binary.ByteOrder
	data  []byte
	tags  map[uint16]entry
}

type entry struct {
	typ   uint16
	count uint32
	raw   []byte // 4 bytes: inline value or file offset
}

// typeSize is the byte size of TIFF field types 1..12.
var typeSize = [...]uint32{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8}

// Decode decodes a GeoTIFF held in memory.
func Decode(data []byte) (*Raster, error) {
	if len(data) < 8 {
		return nil, errors.New("geotiff: file too short")
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, errors.New("geotiff: not a TIFF file")
	}
	switch order.Uint16(data[2:4]) {
	case 42:
	case 43:
		return nil, errors.New("geotiff: BigTIFF is not supported")
	default:
		return nil, errors.New("geotiff: bad TIFF magic number")
	}

	f, err := readIFD(data, order, order.Uint32(data[4:8]))
	if err != nil {
		return nil, err
	}
	return f.decode()
}

func readIFD(data []byte, order binary.ByteOrder, off uint32) (*ifd, error) {
	if uint64(off)+2 > uint64(len(data)) {
		return nil, errors.New("geotiff: IFD offset out of range")
	}
	n := uint64(order.Uint16(data[off:]))
	end := uint64(off) + 2 + n*12
	if end > uint64(len(data)) {
		return nil, errors.New("geotiff: truncated IFD")
	}
	f := &ifd{order: order, data: data, tags: make(map[uint16]entry, n)}
	for i := uint64(0); i < n; i++ {
		e := data[uint64(off)+2+i*12:]
		f.tags[order.Uint16(e)] = entry{
			typ:   order.Uint16(e[2:]),
			count: order.Uint32(e[4:]),
			raw:   e[8:12],
		}
	}
	return f, nil
}

// payload returns the raw bytes of a tag's value.
func (f *ifd) payload(e entry) ([]byte, error) {
	if e.typ == 0 || int(e.typ) >= len(typeSize) {
		return nil, fmt.Errorf("geotiff: unsupported field type %d", e.typ)
	}
	size := uint64(typeSize[e.typ]) * uint64(e.count)
	if size <= 4 {
		return e.raw[:size], nil
	}
	off := uint64(f.order.Uint32(e.raw))
	if off+size > uint64(len(f.data)) {
		return nil, errors.New("geotiff: tag value out of range")
	}
	return f.data[off : off+size], nil
}

// uints returns the integer values of a SHORT/LONG tag.
func (f *ifd) uints(tag uint16) ([]uint64, error) {
	e, ok := f.tags[tag]
	if !ok {
		return nil, nil
	}
	p, err := f.payload(e)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, e.count)
	switch e.typ {
	case 3:
		for i := range out {
			out[i] = uint64(f.order.Uint16(p[i*2:]))
		}
	case 4:
		for i := range out {
			out[i] = uint64(f.order.Uint32(p[i*4:]))
		}
	default:
		return nil, fmt.Errorf("geotiff: tag %d has non-integer type %d", tag, e.typ)
	}
	return out, nil
}

// uint returns a tag's first integer value, or def if the tag is absent.
func (f *ifd) uint(tag uint16, def uint64) (uint64, error) {
	v, err := f.uints(tag)
	if err != nil || len(v) == 0 {
		return def, err
	}
	return v[0], nil
}

func (f *ifd) doubles(tag uint16) ([]float64, error) {
	e, ok := f.tags[tag]
	if !ok {
		return nil, nil
	}
	if e.typ != 12 {
		return nil, fmt.Errorf("geotiff: tag %d is not DOUBLE", tag)
	}
	p, err := f.payload(e)
	if err != nil {
		return nil, err
	}
	out := make([]float64, e.count)
	for i := range out {
		out[i] = math.Float64frombits(f.order.Uint64(p[i*8:]))
	}
	return out, nil
}

func (f *ifd) decode() (*Raster, error) {
	width, err := f.uint(tagImageWidth, 0)
	if err != nil {
		return nil, err
	}
	height, err := f.uint(tagImageLength, 0)
	if err != nil {
		return nil, err
	}
	// Bound the allocation; a DGM1 tile is 1000x1000.
	if width == 0 || height == 0 || width > 1<<16 || height > 1<<16 || width*height > 1<<28 {
		return nil, fmt.Errorf("geotiff: unsupported image size %dx%d", width, height)
	}

	spp, err := f.uint(tagSamplesPerPixel, 1)
	if err != nil {
		return nil, err
	}
	if spp != 1 {
		return nil, fmt.Errorf("geotiff: %d samples per pixel not supported, want 1", spp)
	}
	if planar, err := f.uint(tagPlanarConfig, 1); err != nil || planar != 1 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("geotiff: only chunky planar configuration is supported")
	}

	bits, err := f.uint(tagBitsPerSample, 1)
	if err != nil {
		return nil, err
	}
	format, err := f.uint(tagSampleFormat, 1)
	if err != nil {
		return nil, err
	}
	conv, err := sampleConverter(bits, format, f.order)
	if err != nil {
		return nil, err
	}
	bytesPerSample := int(bits / 8)

	compression, err := f.uint(tagCompression, compressionNone)
	if err != nil {
		return nil, err
	}
	predictor, err := f.uint(tagPredictor, predictorNone)
	if err != nil {
		return nil, err
	}
	switch predictor {
	case predictorNone, predictorHorizontal:
		if predictor == predictorHorizontal && format == 3 {
			return nil, errors.New("geotiff: horizontal predictor on float samples is invalid")
		}
	case predictorFloatingPoint:
		if format != 3 {
			return nil, errors.New("geotiff: floating point predictor on non-float samples")
		}
	default:
		return nil, fmt.Errorf("geotiff: unsupported predictor %d", predictor)
	}

	// Locate the chunks (strips or tiles) that make up the image.
	var offsets, counts []uint64
	var chunkW, chunkH int
	if _, tiled := f.tags[tagTileWidth]; tiled {
		tw, _ := f.uint(tagTileWidth, 0)
		th, _ := f.uint(tagTileLength, 0)
		if tw == 0 || th == 0 || tw > 1<<16 || th > 1<<16 {
			return nil, errors.New("geotiff: bad tile size")
		}
		chunkW, chunkH = int(tw), int(th)
		if offsets, err = f.uints(tagTileOffsets); err != nil {
			return nil, err
		}
		if counts, err = f.uints(tagTileByteCounts); err != nil {
			return nil, err
		}
	} else {
		rps, err := f.uint(tagRowsPerStrip, height)
		if err != nil {
			return nil, err
		}
		if rps == 0 || rps > height {
			rps = height
		}
		chunkW, chunkH = int(width), int(rps)
		if offsets, err = f.uints(tagStripOffsets); err != nil {
			return nil, err
		}
		if counts, err = f.uints(tagStripByteCounts); err != nil {
			return nil, err
		}
	}

	w, h := int(width), int(height)
	across := (w + chunkW - 1) / chunkW
	down := (h + chunkH - 1) / chunkH
	if len(offsets) != across*down || len(counts) != len(offsets) {
		return nil, errors.New("geotiff: chunk offset/count tables do not match image size")
	}

	r := &Raster{Width: w, Height: h, Data: make([]float32, w*h)}
	rowBytes := chunkW * bytesPerSample
	want := rowBytes * chunkH
	buf := make([]byte, 0, want)
	var dec decompressor
	defer dec.close()
	// Float32 samples, the DGM1 layout, skip the per-pixel converter.
	float32Samples := format == 3 && bytesPerSample == 4
	for idx, off := range offsets {
		if off+counts[idx] > uint64(len(f.data)) {
			return nil, errors.New("geotiff: chunk data out of range")
		}
		raw := f.data[off : off+counts[idx]]
		buf, err = dec.decompress(buf[:0], raw, compression, want)
		if err != nil {
			return nil, fmt.Errorf("geotiff: chunk %d: %w", idx, err)
		}
		if len(buf) < want {
			// Strips and tiles at the edge may be stored short; pad rather
			// than reject, but a chunk without even one row is broken.
			if len(buf) < rowBytes {
				return nil, fmt.Errorf("geotiff: chunk %d: short data", idx)
			}
			n := len(buf)
			buf = buf[:want]
			clear(buf[n:])
		}

		x0 := (idx % across) * chunkW
		y0 := (idx / across) * chunkH
		cols := min(chunkW, w-x0)
		rows := min(chunkH, h-y0)

		if float32Samples {
			for row := 0; row < rows; row++ {
				src := buf[row*rowBytes : (row+1)*rowBytes]
				dst := r.Data[(y0+row)*w+x0:][:cols]
				if predictor == predictorFloatingPoint {
					float32RowPredicted(dst, src, chunkW)
				} else {
					float32Row(dst, src, f.order)
				}
			}
			continue
		}

		switch predictor {
		case predictorHorizontal:
			undoHorizontal(buf, chunkH, chunkW, bytesPerSample, f.order)
		case predictorFloatingPoint:
			undoFloatingPoint(buf, chunkH, chunkW, bytesPerSample)
		}
		for row := 0; row < rows; row++ {
			src := buf[row*rowBytes:]
			dst := r.Data[(y0+row)*w:]
			for col := 0; col < cols; col++ {
				dst[x0+col] = conv(src[col*bytesPerSample:], predictor == predictorFloatingPoint)
			}
		}
	}

	if err := f.georeference(r); err != nil {
		return nil, err
	}
	if err := f.applyNoData(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (f *ifd) georeference(r *Raster) error {
	scale, err := f.doubles(tagPixelScale)
	if err != nil {
		return err
	}
	tie, err := f.doubles(tagTiepoint)
	if err != nil {
		return err
	}
	if len(scale) < 2 || len(tie) < 6 {
		return errors.New("geotiff: missing ModelPixelScale/ModelTiepoint (only these georeferencing tags are supported)")
	}
	// Tiepoint maps raster (I, J) to model (X, Y).
	r.OriginX = tie[3] - tie[0]*scale[0]
	r.OriginY = tie[4] + tie[1]*scale[1]
	r.PixelWidth, r.PixelHeight = scale[0], scale[1]
	return nil
}

func (f *ifd) applyNoData(r *Raster) error {
	e, ok := f.tags[tagGDALNoData]
	if !ok {
		return nil
	}
	p, err := f.payload(e)
	if err != nil {
		return err
	}
	s := strings.TrimSpace(strings.TrimRight(string(p), "\x00"))
	if s == "" {
		return nil
	}
	nd, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("geotiff: bad GDAL_NODATA %q", s)
	}
	nan := float32(math.NaN())
	if math.IsNaN(nd) {
		return nil // NaN samples are already NaN
	}
	// Compare in float32: that is what the samples were stored as.
	nd32 := float32(nd)
	for i, v := range r.Data {
		if v == nd32 {
			r.Data[i] = nan
		}
	}
	return nil
}

// sampleConverter returns a function decoding one sample from b. If be is
// true the bytes are big-endian regardless of the file's byte order (the
// floating point predictor always emits most-significant-byte first).
func sampleConverter(bits, format uint64, order binary.ByteOrder) (func(b []byte, be bool) float32, error) {
	pick := func(be bool) binary.ByteOrder {
		if be {
			return binary.BigEndian
		}
		return order
	}
	switch {
	case format == 3 && bits == 32:
		return func(b []byte, be bool) float32 { return math.Float32frombits(pick(be).Uint32(b)) }, nil
	case format == 3 && bits == 64:
		return func(b []byte, be bool) float32 { return float32(math.Float64frombits(pick(be).Uint64(b))) }, nil
	case format == 1 && bits == 8:
		return func(b []byte, _ bool) float32 { return float32(b[0]) }, nil
	case format == 2 && bits == 8:
		return func(b []byte, _ bool) float32 { return float32(int8(b[0])) }, nil
	case format == 1 && bits == 16:
		return func(b []byte, be bool) float32 { return float32(pick(be).Uint16(b)) }, nil
	case format == 2 && bits == 16:
		return func(b []byte, be bool) float32 { return float32(int16(pick(be).Uint16(b))) }, nil
	case format == 1 && bits == 32:
		return func(b []byte, be bool) float32 { return float32(pick(be).Uint32(b)) }, nil
	case format == 2 && bits == 32:
		return func(b []byte, be bool) float32 { return float32(int32(pick(be).Uint32(b))) }, nil
	}
	return nil, fmt.Errorf("geotiff: unsupported sample type (%d-bit, format %d)", bits, format)
}

// decompressor decodes chunks, reusing its zlib reader (about 40 KB of state)
// across the many strips or tiles of one image.
type decompressor struct {
	br bytes.Reader
	zr io.ReadCloser
}

func (d *decompressor) close() {
	if d.zr != nil {
		d.zr.Close()
	}
}

// decompress appends the decoded chunk to dst, reading at most want bytes (a
// chunk never decodes to more). dst must have capacity for want bytes.
func (d *decompressor) decompress(dst, src []byte, compression uint64, want int) ([]byte, error) {
	switch compression {
	case compressionNone:
		return append(dst, src...), nil
	case compressionLZW:
		d.br.Reset(src)
		rc := lzw.NewReader(&d.br, lzw.MSB, 8)
		defer rc.Close()
		return readChunk(dst, rc, want)
	case compressionDeflate, compressionDeflateOld:
		d.br.Reset(src)
		if d.zr == nil {
			zr, err := zlib.NewReader(&d.br)
			if err != nil {
				return nil, err
			}
			d.zr = zr
		} else if err := d.zr.(zlib.Resetter).Reset(&d.br, nil); err != nil {
			return nil, err
		}
		return readChunk(dst, d.zr, want)
	}
	return nil, fmt.Errorf("unsupported compression %d", compression)
}

// readChunk reads up to want bytes from rc into dst's spare capacity. A short
// stream is not an error here: the caller decides whether it holds enough.
func readChunk(dst []byte, rc io.Reader, want int) ([]byte, error) {
	if cap(dst) < want {
		dst = make([]byte, 0, want)
	}
	dst = dst[:want]
	n, err := io.ReadFull(rc, dst)
	if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return dst[:n], nil
}

// float32Row decodes one row of float32 samples stored in the file's byte order.
func float32Row(dst []float32, src []byte, order binary.ByteOrder) {
	switch order {
	case binary.LittleEndian:
		for c := range dst {
			dst[c] = math.Float32frombits(binary.LittleEndian.Uint32(src[c*4:]))
		}
	case binary.BigEndian:
		for c := range dst {
			dst[c] = math.Float32frombits(binary.BigEndian.Uint32(src[c*4:]))
		}
	default:
		for c := range dst {
			dst[c] = math.Float32frombits(order.Uint32(src[c*4:]))
		}
	}
}

// float32RowPredicted undoes TIFF predictor 3 (Adobe TN3) for one row of
// float32 samples and decodes it into dst. The row is byte-wise delta encoded
// and stored as four byte planes of cols bytes each, most significant first;
// dst may be shorter than cols when the chunk overhangs the image edge.
func float32RowPredicted(dst []float32, row []byte, cols int) {
	// Carry the running sum in a register: reloading row[i-1] each step
	// makes the loop wait on the store it just issued.
	var sum byte
	for i, b := range row {
		sum += b
		row[i] = sum
	}
	n := len(dst)
	p0, p1, p2, p3 := row[:n], row[cols:cols+n], row[2*cols:2*cols+n], row[3*cols:3*cols+n]
	for c := range dst {
		dst[c] = math.Float32frombits(uint32(p0[c])<<24 | uint32(p1[c])<<16 | uint32(p2[c])<<8 | uint32(p3[c]))
	}
}

// undoHorizontal reverses TIFF predictor 2 (per-row differencing of samples).
func undoHorizontal(buf []byte, rows, cols, bytesPerSample int, order binary.ByteOrder) {
	rowBytes := cols * bytesPerSample
	for r := 0; r < rows; r++ {
		row := buf[r*rowBytes : (r+1)*rowBytes]
		switch bytesPerSample {
		case 1:
			for i := 1; i < cols; i++ {
				row[i] += row[i-1]
			}
		case 2:
			for i := 1; i < cols; i++ {
				order.PutUint16(row[i*2:], order.Uint16(row[i*2:])+order.Uint16(row[(i-1)*2:]))
			}
		case 4:
			for i := 1; i < cols; i++ {
				order.PutUint32(row[i*4:], order.Uint32(row[i*4:])+order.Uint32(row[(i-1)*4:]))
			}
		}
	}
}

// undoFloatingPoint reverses TIFF predictor 3 (Adobe TN3): each row is
// byte-wise delta encoded and stored as byte planes, most significant first.
// The result is each sample as big-endian bytes.
func undoFloatingPoint(buf []byte, rows, cols, bytesPerSample int) {
	rowBytes := cols * bytesPerSample
	tmp := make([]byte, rowBytes)
	for r := 0; r < rows; r++ {
		row := buf[r*rowBytes : (r+1)*rowBytes]
		for i := 1; i < rowBytes; i++ {
			row[i] += row[i-1]
		}
		copy(tmp, row)
		for c := 0; c < cols; c++ {
			for b := 0; b < bytesPerSample; b++ {
				row[c*bytesPerSample+b] = tmp[b*cols+c]
			}
		}
	}
}
