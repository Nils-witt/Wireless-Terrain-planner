#!/bin/sh
# Regenerates the GeoTIFF fixtures used by geotiff_test.go. Needs python3 with
# numpy, and GDAL's gdal_translate. The tests compare each fixture against
# expected_f32.raw / expected_i16.raw (little-endian, row-major, 53x37).
set -eu
cd "$(dirname "$0")"

python3 - <<'PY'
import numpy as np
h, w = 37, 53  # deliberately not multiples of the 16px tile / 10-row strip sizes
rng = np.random.default_rng(1)
a = (100 + 0.37*np.arange(w)[None, :] + 0.81*np.arange(h)[:, None] + rng.normal(0, 0.5, (h, w))).astype('<f4')
a[5, 7] = -9999
a[20:22, 30:33] = -9999
i = np.rint(a).astype('<i2')
a.tofile("expected_f32.raw")
i.tofile("expected_i16.raw")
def asc(name, arr, fmt):
    with open(name, "w") as f:
        f.write(f"ncols {w}\nnrows {h}\nxllcorner 350000\nyllcorner 5700000\ncellsize 1\nNODATA_value -9999\n")
        for row in arr:
            f.write(" ".join(format(v, fmt) for v in row) + "\n")
asc("f32.asc", a, ".9g")
asc("i16.asc", i, "d")
PY

T="-a_srs EPSG:25832 -q"
gdal_translate $T -ot Float32 -co COMPRESS=NONE            f32.asc f32_none.tif
gdal_translate $T -ot Float32 -co COMPRESS=LZW             f32.asc f32_lzw.tif
gdal_translate $T -ot Float32 -co COMPRESS=LZW -co PREDICTOR=3 -co BLOCKYSIZE=10 f32.asc f32_lzw_pred3_strips.tif
gdal_translate $T -ot Float32 -co COMPRESS=DEFLATE -co PREDICTOR=3 -co TILED=YES -co BLOCKXSIZE=16 -co BLOCKYSIZE=16 f32.asc f32_deflate_pred3_tiled.tif
gdal_translate $T -ot Float32 -co COMPRESS=DEFLATE -co BLOCKYSIZE=10 f32.asc f32_deflate_strips.tif
gdal_translate $T -ot Float32 -co COMPRESS=LZW -co PREDICTOR=3 -co ENDIANNESS=BIG f32.asc f32_lzw_pred3_be.tif
gdal_translate $T -ot Int16   -co COMPRESS=LZW -co PREDICTOR=2 i16.asc i16_lzw_pred2.tif
gdal_translate $T -ot Int16   -co COMPRESS=DEFLATE -co PREDICTOR=2 -co ENDIANNESS=BIG -co TILED=YES -co BLOCKXSIZE=16 -co BLOCKYSIZE=16 i16.asc i16_deflate_pred2_be_tiled.tif
rm -f f32.asc i16.asc *.aux.xml
