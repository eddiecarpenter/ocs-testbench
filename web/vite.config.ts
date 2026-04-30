/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vite.dev/config/
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { storybookTest } from '@storybook/addon-vitest/vitest-plugin';
import { playwright } from '@vitest/browser-playwright';
const dirname = typeof __dirname !== 'undefined' ? __dirname : path.dirname(fileURLToPath(import.meta.url));

// Go backend dev port — override with VITE_BACKEND_URL when your local
// server listens elsewhere. Defaults to http://localhost:8080 (the
// common convention for Go HTTP dev servers). Used for the `/api`
// dev-server proxy below; production builds are embedded into the
// binary via `go:embed` and served from the same origin, so the proxy
// is a dev-time-only concern.
const BACKEND_URL = process.env.VITE_BACKEND_URL ?? 'http://localhost:8080';

// HMR client port — when the developer is running the Go binary with
// `-tags dev`, the browser hits the Go process on :8080 and Go reverse-
// proxies SPA requests to Vite on :5173. The HMR WebSocket goes through
// the same proxy, so the browser must connect back to :8080 (the user-
// facing port), NOT :5173 (the Vite-internal port). Without this, Vite
// generates HMR URLs pointing at :5173, which the browser can't reach
// behind the proxy.
//
// Set VITE_HMR_CLIENT_PORT=5173 (or unset) when running `npm run dev`
// standalone — i.e. browsing Vite directly at :5173 without the Go
// proxy in front.
const HMR_CLIENT_PORT = Number(process.env.VITE_HMR_CLIENT_PORT ?? 8080);

// More info at: https://storybook.js.org/docs/next/writing-tests/integrations/vitest-addon
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: BACKEND_URL,
        changeOrigin: true,
      },
    },
    hmr: {
      // See HMR_CLIENT_PORT above.
      clientPort: HMR_CLIENT_PORT,
    },
  },
  test: {
    projects: [{
      extends: true,
      plugins: [
      // The plugin will run tests for the stories defined in your Storybook config
      // See options at: https://storybook.js.org/docs/next/writing-tests/integrations/vitest-addon#storybooktest
      storybookTest({
        configDir: path.join(dirname, '.storybook')
      })],
      test: {
        name: 'storybook',
        browser: {
          enabled: true,
          headless: true,
          provider: playwright({}),
          instances: [{
            browser: 'chromium'
          }]
        }
      }
    }]
  }
});
