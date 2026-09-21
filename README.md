# pg — elevation profile explorer

Small web + Go backend project for generating elevation profiles and visualizing maps.

This repository contains a lightweight frontend (Vite + TypeScript) and a Go backend that computes elevation profiles from geospatial tile data. A proxy (nginx) is used in front of the services when running with Docker Compose.

## Repository layout

- `frontend/` — Vite + TypeScript single-page app. `npm run build` writes to `internal/web/dist`, which is embedded into the server binary.
- `server/` — Go HTTP server and elevation profile code. See `cmd` (entry point) and `internal`. `internal/web` embeds and serves the built frontend.
- `proxy/` — `nginx.conf` used by the `proxy` service in the Compose setups.
- `compose.yaml` — Docker Compose file that builds the local `frontend` and `server` images and runs `proxy`.
- `compose-ghcr.yaml` — Docker Compose file that references prebuilt images hosted on GHCR.

## Prerequisites

- Node.js (recommended latest LTS), npm or yarn — for the frontend
- Go 1.26 or newer — for local backend dev (see `go.mod`)
- Docker & Docker Compose (for containerized runs)

## Quickstart — Docker Compose (local build)

This will build the `frontend` and `server` images from the repository and start the `proxy`, `frontend` and `app` services.

```bash
# from project root
docker compose -f compose.yaml up --build
```

The `proxy` service exposes port 80 on the host. The nginx configuration in `proxy/nginx.conf` proxies requests to the frontend and backend containers.

## Quickstart — Docker Compose (GHCR images)

If you prefer to use prebuilt images published to GHCR, run:

```bash
docker compose -f compose-ghcr.yaml up
```

## Local development

Frontend

```bash
cd frontend
# install dependencies (npm example)
npm install
# start Vite dev server
npm run dev
```

The frontend dev server runs on Vite's default port (usually 5173). Open the URL printed by Vite in your browser. It proxies `/api` to the Go server at `http://localhost:8000` (the server's default `LISTEN_ADDR`), so start the backend (see below) alongside it. Set `API_TARGET` to point the proxy elsewhere, e.g. `API_TARGET=http://localhost:9000 npm run dev`.

Backend (Go)

```bash
# build the frontend once; the server embeds the result (internal/web/dist)
(cd frontend && npm install && npm run build)
TILE_DIR=./dgm1_tiff_kacheln LISTEN_ADDR=:5001 go run ./cmd/server
# run the tests
go test ./...
```

The UI is then at `http://localhost:5001/` and the API at `/api`. The frontend is embedded at compile time, so rebuild it and restart the server to pick up frontend changes. Without a frontend build the server still starts and serves only `/api` (it logs a warning); `go build`, `go vet` and `go test` work without one.

The server is configured through environment variables:

| Variable          | Default               | Meaning                                                   |
|-------------------|-----------------------|-----------------------------------------------------------|
| `LISTEN_ADDR`     | `:8000`               | Address to listen on                                      |
| `TILE_DIR`        | `/dgm1_tiff_kacheln`  | Directory containing the `.tif` elevation tiles           |
| `TILE_CACHE_SIZE` | `32`                  | Decoded tiles kept in memory (about 4 MB each)            |

Notes about Dockerfile: the root `Dockerfile` is a multi-stage build: it builds the frontend with Node, compiles a static Go binary that embeds it, and copies the binary into a minimal distroless image. Build it from the repository root (no GDAL or other system libraries needed). The container listens on port 8000 and runs as a non-root user, so the mounted tile directory must be world-readable. The compose files rely on the nginx proxy for routing, so you usually do not need to expose the app directly.

## API (simple example)

The server exposes a single API endpoint, `/api`, that accepts four query parameters and returns an elevation profile. Parameters:

- `latitude1`, `longitude1` — first coordinate (lat/lon)
- `latitude2`, `longitude2` — second coordinate (lat/lon)

The response is a JSON array of `[distance_m, elevation_m]` pairs sampled every metre along the line, ordered by distance. Invalid or missing parameters return `400` with a JSON `{"error": "..."}` body.

Example request (Docker image with port 8000 published on the host port 80):

```bash
curl "http://localhost/api?latitude1=52.0&longitude1=13.0&latitude2=52.1&longitude2=13.1"
```

If you're running the Go server directly with `LISTEN_ADDR=:5001` (see above), point at port 5001:

```bash
curl "http://localhost:5001/api?latitude1=52.0&longitude1=13.0&latitude2=52.1&longitude2=13.1"
```

## Data

- The compose files mount `./dgm1_tiff_kacheln` to `/dgm1_tiff_kacheln` in the app container. Place your elevation tiles there. Tiles must be DGM1 GeoTIFFs in EPSG:25832 named `dgm1_32_<easting km>_<northing km>_1_nw_2021.tif`; the server reads them itself (no GDAL required) and supports uncompressed, LZW and Deflate compression.

## Build & CI notes

- Frontend build: `cd frontend && npm run build` — this runs `tsc && vite build` as defined in `frontend/package.json` and writes to `internal/web/dist` (git-ignored except for a `.gitkeep` placeholder).
- Server build: `go build ./cmd/server` (build the frontend first to embed it) — CI runs `go vet ./...` and `go test -race -shuffle=on ./...`, plus `golangci-lint` (`.golangci.yml`) and `govulncheck`. The Dockerfile builds a static binary.
- Releases: pushing a `v*` tag runs GoReleaser (`.goreleaser.yaml`), which builds the frontend, cross-compiles the server for linux/darwin/windows on amd64/arm64 and attaches the archives and `checksums.txt` to a GitHub Release. Pull requests that touch the code run the same build as a snapshot without publishing. To try it locally: `goreleaser release --snapshot --clean --skip=publish`.

## Troubleshooting

- If the server logs `tile not found`, check that `TILE_DIR` points at the tiles and that the file names match the pattern above. Samples in missing or unreadable tiles are left out of the profile.
- If the frontend does not reach the backend when running with Docker Compose, check `proxy/nginx.conf` and that the compose stack is healthy (`docker compose ps`).


# Copyright
Copyright [2025] [Nils Witt]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
