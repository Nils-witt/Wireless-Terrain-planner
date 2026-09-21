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
