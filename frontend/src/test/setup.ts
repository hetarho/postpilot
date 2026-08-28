import '@testing-library/jest-dom/vitest'

// jsdom has no layout engine, so every router navigation would otherwise log
// "Not implemented: Window's scrollTo()" and bury the real test output.
window.scrollTo = () => {}
