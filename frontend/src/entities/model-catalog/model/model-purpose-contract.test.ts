import { describe, expect, it } from 'vitest'

import { ProtoModelPurpose } from '@/shared/api'

import { MODEL_PURPOSES, type ModelPurpose } from '../config'
import { isModelPurpose } from './types'

const protoToDomain = new Map<ProtoModelPurpose, ModelPurpose | undefined>([
  [ProtoModelPurpose.UNSPECIFIED, undefined],
  [ProtoModelPurpose.PHOTO_ANALYSIS, 'photo-analysis'],
  [ProtoModelPurpose.STYLE_ANALYSIS, 'style-analysis'],
  [ProtoModelPurpose.WRITING, 'writing'],
  [ProtoModelPurpose.IMAGE_GENERATION, 'image-generation'],
  [ProtoModelPurpose.VIDEO_GENERATION, 'video-generation'],
])

describe('model purpose enum contract', () => {
  it('pins every generated value to the frontend purpose mirror', () => {
    const generated = Object.values(ProtoModelPurpose).filter(
      (value): value is ProtoModelPurpose => typeof value === 'number',
    )

    expect(protoToDomain.size).toBe(generated.length)
    expect(generated.map((value) => protoToDomain.get(value)).filter(Boolean)).toEqual([
      ...MODEL_PURPOSES,
    ])
    for (const value of generated) {
      expect(protoToDomain.has(value)).toBe(true)
      const purpose = protoToDomain.get(value)
      expect(purpose ? isModelPurpose(purpose) : false).toBe(
        value !== ProtoModelPurpose.UNSPECIFIED,
      )
    }
  })

  it('does not turn unspecified or unknown values into a valid purpose', () => {
    expect(protoToDomain.get(ProtoModelPurpose.UNSPECIFIED)).toBeUndefined()
    expect(protoToDomain.get(999 as ProtoModelPurpose)).toBeUndefined()
    expect(isModelPurpose('')).toBe(false)
    expect(isModelPurpose('translation')).toBe(false)
  })
})
