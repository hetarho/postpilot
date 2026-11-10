import i18next from 'i18next'
import { resources } from '@/app/providers/i18n/resources'

// `setup.ts` initializes i18next from the catalogs `i18n-global-setup.ts` evaluated once for the
// run instead of importing them in every file. This file imports the catalogs itself, so a key
// or value that differs between the two — a catalog edited without reaching the snapshot, or a
// value lost in transfer — fails here.
it('initializes every DOM test file with the current catalogs', () => {
  expect(i18next.store.data).toStrictEqual(resources)
  expect(i18next.language).toBe('ko')
})
