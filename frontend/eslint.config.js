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
  // ARCH-17: proto service descriptors, message schemas and `@connectrpc/connect-query` are
  // named only in `shared/api` and `entities/*/api`. Pages, widgets and features consume the
  // hooks and domain types an entity exports, so a proto rename stops at one directory and a
  // message rename at one entity.
  {
    files: ['src/pages/**/*.{ts,tsx}', 'src/widgets/**/*.{ts,tsx}', 'src/features/**/*.{ts,tsx}'],
    // Tests are out of scope here for the same reason steiger skips them: a fake backend is
    // built from `createRouterTransport` and the service descriptor, and `src/test` owns those
    // harnesses. The source-tree pin in `src/test/arch-proto-symbols.test.ts` states the rule
    // for slice source.
    ignores: ['src/**/*.test.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: '@connectrpc/connect-query',
              message:
                'connect-query는 shared/api와 entities/*/api에서만 써요. 페이지·위젯·피처는 엔티티가 내보낸 훅을 쓰세요 (ARCH-17).',
            },
            {
              name: '@connectrpc/connect',
              message:
                'Connect 클라이언트는 entities/*/api가 만들어요. 페이지·위젯·피처는 엔티티 훅을 쓰세요 (ARCH-17).',
            },
          ],
        },
      ],
      'no-restricted-syntax': [
        'error',
        {
          selector:
            "ImportDeclaration[source.value='@/shared/api'] > ImportSpecifier[imported.name=/(Service|Schema)$|^Proto[A-Z]/]",
          message:
            'proto 서비스·메시지 스키마 이름은 shared/api와 entities/*/api에만 있어야 해요. 엔티티가 도메인 타입으로 바꿔서 내보내세요 (ARCH-17).',
        },
        {
          selector:
            'ImportDeclaration[source.value=/^@\\/entities\\//] > ImportSpecifier[imported.name=/(ToProto|FromProto)$/]',
          message:
            '엔티티의 wire 매퍼는 엔티티 안에서만 써요. 엔티티가 도메인 값을 받는 훅을 내보내게 하세요 (ARCH-17).',
        },
      ],
    },
  },
  // Formatting is delegated to Prettier (must stay last) — disables conflicting rules.
  eslintConfigPrettier,
])
