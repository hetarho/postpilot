import '@testing-library/jest-dom/vitest'
import { configure } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'

initializeI18n('ko')

// The suite runs 70+ files in parallel, so a page-level `findBy*` can exceed testing-library's
// 1s default from scheduling pressure alone rather than from anything being wrong — PostsPage's
// first render did, on roughly half of full-suite runs, long before this line existed. Only the
// deadline moves; every assertion stays exactly as strict, and a genuinely broken query still
// fails, just later.
configure({ asyncUtilTimeout: 5_000 })

// jsdom has no layout engine, so every router navigation would otherwise log
// "Not implemented: Window's scrollTo()" and bury the real test output.
window.scrollTo = () => {}
