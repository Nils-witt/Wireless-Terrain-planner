# Wireless Terrain Planner

Plan a point-to-point radio link on a map. Place two antennas, and the app draws the terrain between them, checks whether the line of sight is blocked, and estimates the received signal and range.

It is a single Go binary. The server computes elevation profiles from 1 m DGM1 GeoTIFF tiles, and it serves the web UI embedded in the binary. The UI is a Vite + TypeScript app built on MapLibre GL and Chart.js.

## Features

- Right-click the map to set the position of **Antenna One** or **Antenna Two**.
- Per antenna: frequency (MHz), transmit power (dBm), antenna gain (dBi), mast height (m) and receiver sensitivity.
- **Terrain chart** between the two antennas, with the line of sight drawn from antenna tip to antenna tip and the points where terrain rises above it marked.
- **Expected signal** and **range circle** per antenna, using the Friis free-space model. Terrain diffraction is not modelled, so treat the numbers as an optimistic upper bound.
- Distance and azimuth between the antennas.

The base map is OpenStreetMap raster tiles loaded from `tile.openstreetmap.org` in the browser, so the OSM [tile usage policy](https://operations.osmfoundation.org/policies/tiles/) applies.

## Quick start

### Docker

The tiles are not part of the repository (see [Elevation data](#elevation-data)). Mount them into the container:

```bash
docker build -t wireless-terrain-planner .
docker run --rm -p 8000:8000 \
  -v "$PWD/dgm1_tiff_kacheln:/dgm1_tiff_kacheln:ro" \
  wireless-terrain-planner
```

Open <http://localhost:8000>. The image is multi-stage: it builds the frontend with Node, compiles a static Go binary that embeds it, and ships that binary in a distroless image. The container runs as a non-root user, so the tile files must be world-readable.

### From source

You need Go 1.26+ and Node.js (CI uses 26.x). No GDAL or other system libraries are required.

```bash
# build the frontend first; the server embeds the result (internal/web/dist)
(cd frontend && npm ci && npm run build)

TILE_DIR=./dgm1_tiff_kacheln LISTEN_ADDR=:5001 go run ./cmd/server
```

Open <http://localhost:5001>. The frontend is embedded at compile time, so rebuild it and restart the server to pick up frontend changes. Without a frontend build the server still starts and serves only `/api`, and it logs a warning.

### Prebuilt binaries

Tagged releases attach archives for Linux, macOS and Windows (amd64 and arm64) to the [GitHub Releases](../../releases) page, built by GoReleaser. Each one is a standalone binary with the UI included.

## Elevation data

The server reads **DGM1** GeoTIFF tiles from `TILE_DIR`. Each tile covers 1 km × 1 km at 1 m resolution, in EPSG:25832 (ETRS89 / UTM zone 32N), and is named after its south-west corner:

```
dgm1_32_<easting km>_<northing km>_1_nw_2021.tif      e.g. dgm1_32_360_5613_1_nw_2021.tif
```

- Only the tiles you provide are used. A profile that crosses a missing tile has no samples there, so download the area you want to plan in.
- The reader is built in. It handles uncompressed, LZW and Deflate GeoTIFFs, with strips or tiles.
- Points are projected to UTM zone 32N, so coverage is limited to data in that zone.
- The tile directory is git-ignored.

## Configuration

The server is configured through environment variables:

| Variable          | Default             | Meaning                                                                                                          |
| ----------------- | ------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `LISTEN_ADDR`     | `:8000`             | Address to listen on                                                                                             |
| `TILE_DIR`        | `dgm1_tiff_kacheln` | Directory containing the `.tif` tiles, relative to the working directory (`/dgm1_tiff_kacheln` in the container) |
| `TILE_CACHE_SIZE` | `32`                | Decoded tiles kept in memory (about 4 MB each)                                                                   |

## API

`GET /api` returns the elevation profile of the straight line between two WGS84 points.

| Query parameter           | Meaning                 |
| ------------------------- | ----------------------- |
| `latitude1`, `longitude1` | Start point, in degrees |
| `latitude2`, `longitude2` | End point, in degrees   |

All four are required. Latitude must be within ±90 and longitude within ±180.

```bash
curl "http://localhost:8000/api?latitude1=50.6596&longitude1=7.1731&latitude2=50.6650&longitude2=7.1900"
```

The response is a JSON array of `[distance_m, elevation_m]` pairs, ordered by distance from the start point and sampled about every metre. It grows with the length of the link, at roughly 40 bytes per metre.

```json
[[0, 102.42], [1.0001, 102.41], [2.0002, 102.36], "..."]
```

Samples that fall in a missing tile, on a nodata value, or in the last row or column of a tile are left out, so the array can have gaps. An invalid or missing parameter returns `400` with a JSON body listing every problem:

```json
{"error": "query parameter \"latitude1\" must be between -90 and 90\nmissing query parameter \"latitude2\""}
```

## Development

### Repository layout

| Path               | Contents                                                                                     |
| ------------------ | -------------------------------------------------------------------------------------------- |
| `cmd/server`       | Entry point: config, HTTP server, graceful shutdown                                          |
| `internal/httpapi` | The `/api` handler and parameter validation                                                  |
| `internal/profile` | Samples the line and interpolates elevations; LRU tile cache; loads crossed tiles in parallel |
| `internal/geotiff` | Minimal GeoTIFF reader                                                                       |
| `internal/utm`     | WGS84 to EPSG:25832 projection                                                               |
| `internal/web`     | Embeds and serves the built frontend (`internal/web/dist`)                                   |
| `frontend/`        | Vite + TypeScript app                                                                        |

### Frontend

```bash
cd frontend
npm ci
npm run dev
```

Vite serves the app on port 5173 and proxies `/api` to `http://localhost:8000`, so run the Go server alongside it. Point the proxy elsewhere with `API_TARGET`, for example `API_TARGET=http://localhost:9000 npm run dev`.

`npm run build` type-checks with `tsc` and writes the bundle to `internal/web/dist`. That directory is git-ignored apart from a `.gitkeep` placeholder, which lets the Go packages compile without a frontend build. The build restores the placeholder after it empties the directory.

### Backend

```bash
go test ./...
go vet ./...
```

Both work without a frontend build. Benchmarks for the handler, profile sampler and GeoTIFF reader live next to the tests (`go test -bench . ./internal/...`).

### Checks that CI runs

| Area     | Checks                                                                                 |
| -------- | -------------------------------------------------------------------------------------- |
| Frontend | `npx tsc`, `npm run lint` (oxlint), `npm run format:check` (Prettier)                  |
| Go       | `go test -race -shuffle=on ./...`, `go mod tidy -diff`, `golangci-lint`, `govulncheck` |

Run `npm run format` in `frontend/` to apply Prettier. Dependabot keeps dependencies up to date.

### Releases

Pushing a `v*` tag runs GoReleaser (`.goreleaser.yaml`). It builds the frontend, cross-compiles the server and attaches the archives and `checksums.txt` to a GitHub Release. Pull requests that touch the code run the same build as a snapshot without publishing. To try it locally:

```bash
goreleaser release --snapshot --clean --skip=publish
```

## Troubleshooting

- **Empty chart, or a `tile not found` warning in the server log.** `TILE_DIR` does not point at the tiles, the file names do not match the pattern above, or the antennas are outside the area you have tiles for.
- **The UI shows a 404 and the log says `serving the API only`.** The frontend was not built before the server. Run `npm run build` in `frontend/` and restart.
- **Permission errors in Docker.** The container runs as a non-root user, so the mounted tile directory and files must be readable by everyone.
- **The map is blank.** The browser cannot reach `tile.openstreetmap.org`.

## License

Copyright 2026 Nils Witt

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

<http://www.apache.org/licenses/LICENSE-2.0>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
