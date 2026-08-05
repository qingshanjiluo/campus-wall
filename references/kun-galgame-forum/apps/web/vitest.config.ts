// Nuxt + Vitest integration.
//
// @nuxt/test-utils/config wraps Vite + Nuxt's module graph so tests can
// import composables / utils / components by their auto-import names
// (the same way SFCs do), with the Nuxt environment available when a
// test needs it (mount components, access useRoute, etc.). For pure
// utility tests we keep environment="happy-dom" — much faster than
// spinning up a full Nuxt sandbox per test file.
//
// Two environments coexist:
//   - environment: 'happy-dom'  → pure ts/js utils, lightweight
//   - // @vitest-environment nuxt  (per-file pragma) → component + composable
//
// Run:  pnpm test            # one-shot
//       pnpm test:watch      # re-run on save
import { defineVitestConfig } from '@nuxt/test-utils/config'

export default defineVitestConfig({
  test: {
    environment: 'happy-dom',
    globals: true,
    // colocated specs: any *.spec.ts next to source + the standalone
    // tests/ tree for cross-cutting cases.
    include: [
      'app/**/*.spec.ts',
      'shared/**/*.spec.ts',
      'tests/**/*.spec.ts'
    ],
    exclude: ['**/node_modules/**', '**/dist/**', '**/.nuxt/**']
  }
})
