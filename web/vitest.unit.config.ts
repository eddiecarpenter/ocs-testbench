/**
 * Unit-test configuration for the web app.
 *
 * Lives alongside `vite.config.ts` (which is reserved for the
 * Storybook visual test runner that needs Playwright) and is
 * standalone so unit tests can run without spinning up Playwright /
 * Chromium. Tests are picked up under `src/` matching the standard
 * vitest globs (`*.test.ts`, `*.test.tsx`).
 */
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    name: 'unit',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    environment: 'node',
    globals: false,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/features/**'],
    },
  },
});
