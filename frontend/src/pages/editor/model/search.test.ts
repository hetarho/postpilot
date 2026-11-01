import { expect, it } from 'vitest'
import { editorSearchSchema } from './search'

// TMPL-32: the wizard hands back only the template it saved.
it('keeps a handed template and drops anything else', () => {
  expect(editorSearchSchema({ template: 'template-1' })).toEqual({ template: 'template-1' })
  expect(editorSearchSchema({ template: ' ' })).toEqual({ template: undefined })
  expect(editorSearchSchema({ template: 3, other: 'x' })).toEqual({ template: undefined })
})

// POST-121: first-use setup's finish is the one other thing a new post is told.
it('keeps the setup handoff and only that literal', () => {
  expect(editorSearchSchema({ from: 'setup' })).toMatchObject({ from: 'setup' })
  expect(editorSearchSchema({ from: 'home' })).toMatchObject({ from: undefined })
})
