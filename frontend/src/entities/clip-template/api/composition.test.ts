import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { VideoTemplateSchema } from '@/shared/api'
import { toClipTemplate } from './clip-template'
import { recipeOf, validateClipRecipe } from '../model/types'

describe('portable template projection', () => {
  it('keeps exact source and accepts duplicate display labels with distinct field IDs', () => {
    const body =
      '\n<clip version="1"><field id="a" label="가격"/><field id="b" label="가격"/><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>\n'
    const value = toClipTemplate(
      create(VideoTemplateSchema, {
        name: 'source',
        compositionBody: body,
      }),
    )
    expect(recipeOf(value).compositionBody).toBe(body)
    expect(validateClipRecipe(value).valid).toBe(true)
    expect(validateClipRecipe({ ...value, compositionBody: '<clip/>' }).valid).toBe(false)
  })
})
