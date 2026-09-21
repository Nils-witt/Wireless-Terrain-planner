import {writeFileSync} from 'node:fs';

// The Go server embeds the build output (see internal/web).
const outDir = '../internal/web/dist';

// The dev server forwards /api to the Go server (LISTEN_ADDR default :8000).
const apiTarget = process.env.API_TARGET ?? 'http://localhost:8000';

export default {
    server: {
        proxy: {
            '/api': {
                target: apiTarget,
                changeOrigin: true,
            },
        },
    },
    build: {
        outDir,
        emptyOutDir: true,
        // The MapLibre chunk is ~1 MB by itself; anything past this means app code is bloating.
        chunkSizeWarningLimit: 1100,
        rolldownOptions: {
            output: {
                codeSplitting: {
                    // MapLibre dominates the bundle and changes rarely; keeping it in its own
                    // chunk lets browsers reuse it from cache when only app code changes.
                    groups: [{name: 'maplibre', test: /node_modules[\\/]maplibre-gl/}],
                },
            },
        },
    },
    plugins: [
        {
            // emptyOutDir removes the committed placeholder that lets the Go
            // packages compile without a frontend build; put it back.
            name: 'keep-embed-placeholder',
            closeBundle() {
                writeFileSync(new URL(`${outDir}/.gitkeep`, import.meta.url), '');
            },
        },
    ],
};
