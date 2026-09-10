import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ModelInfoSchema } from '@/shared/api'
import { toCatalogModel } from './catalog-mappers'

it('keeps raw video capability separate from each workflow transport', () => {
  const model = toCatalogModel(
    create(ModelInfoSchema, {
      videoInput: true,
      signedVideoUrl: false,
      inlineStaticVideo: true,
    }),
  )
  expect(model).toMatchObject({ videoInput: true, signedVideoUrl: false, inlineStaticVideo: true })
  expect(toCatalogModel(create(ModelInfoSchema, { videoInput: true }))).toMatchObject({
    videoInput: true,
    signedVideoUrl: false,
    inlineStaticVideo: false,
  })
})
