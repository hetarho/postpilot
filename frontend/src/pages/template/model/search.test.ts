import { expect, it } from 'vitest'
import { newTemplateSearchSchema } from './search'

// TMPL-32: the post whose 템플릿 picker opened the wizard, which 저장 returns to.
it('keeps the post a new template returns to', () => {
  expect(newTemplateSearchSchema({ post: '20260820-memo' })).toMatchObject({
    post: '20260820-memo',
  })
  expect(newTemplateSearchSchema({ post: 'new' })).toMatchObject({ post: 'new' })
  expect(newTemplateSearchSchema({ post: '' })).toMatchObject({ post: undefined })
})
