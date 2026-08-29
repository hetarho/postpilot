import { defineConfig } from 'steiger'
import fsd from '@feature-sliced/steiger-plugin'

// Feature-Sliced Design structure linter — import direction, public API (a slice is
// reachable only through its index.ts), cross-imports (@x), segment/slice naming.
// (Pure-layer purity is ESLint boundaries' job.) Run with `pnpm lint:fsd`.
export default defineConfig([
  ...fsd.configs.recommended,
  {
    // Generated code and tests are not subject to FSD rules.
    ignores: ['**/shared/api/gen/**', '**/*.test.{ts,tsx}', '**/__mocks__/**'],
  },
  {
    rules: {
      // The scaffold has a single page; "this slice is only referenced once" is expected
      // until real features land.
      'fsd/insignificant-slice': 'off',
      // Product actions intentionally remain separate verb slices. A global slice-count
      // heuristic cannot distinguish that placement from accidental fragmentation; the
      // dependency/public-API rules below remain enforced.
      'fsd/excessive-slicing': 'off',
    },
  },
])
