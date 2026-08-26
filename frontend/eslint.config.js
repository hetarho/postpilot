import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import eslintConfigPrettier from 'eslint-config-prettier'
import boundaries from 'eslint-plugin-boundaries'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  // src/shared/api/gen is protoc-gen-es output (generated transport types) —
  // infrastructure, neither linted nor hand-edited.
  globalIgnores(['dist', 'coverage', 'src/shared/api/gen']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
  },
  // Architecture boundaries: the pure layers (entities/*/model, shared/lib, shared/api,
  // shared/config) hold framework-free logic and must not import react/react-dom, so they
  // stay portable (tests, workers, a future native client). Rendering code belongs in */ui.
  // (FSD structure rules — import direction, public API, segment names — are steiger's job.)
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { boundaries },
    settings: {
      'boundaries/include': ['src/**/*.{ts,tsx}'],
      'boundaries/elements': [
        {
          type: 'pure',
          mode: 'full',
          pattern: [
            'src/entities/*/model/**/*',
            'src/shared/lib/**/*',
            'src/shared/api/**/*',
            'src/shared/config/**/*',
          ],
        },
        // Everything else is a platform layer (ui/app/pages/widgets/features) — React OK.
        { type: 'platform', mode: 'full', pattern: ['src/**/*'] },
      ],
    },
    rules: {
      'boundaries/dependencies': [
        'error',
        {
          checkAllOrigins: true,
          default: 'allow',
          rules: [
            {
              from: { type: 'pure' },
              disallow: {
                to: { origin: 'external' },
                dependency: { module: ['react', 'react-dom'] },
              },
              message:
                '순수 레이어(model/shared)는 react/react-dom을 import할 수 없어요. 렌더링 코드는 */ui로 옮기세요.',
            },
          ],
        },
      ],
    },
  },
  // Formatting is delegated to Prettier (must stay last) — disables conflicting rules.
  eslintConfigPrettier,
])
