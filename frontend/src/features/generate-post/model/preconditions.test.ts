import { describe, expect, it } from 'vitest'
import { deletedVoiceAIReason } from '@/entities/voice'
import {
  briefIssues,
  comparisonGenerationPreconditions,
  isSetupBlocker,
  ordinaryGenerationPreconditions,
  setupBlockerTarget,
  type GenerationModelSelection,
} from './preconditions'

const image = { id: 'image-1' }
const vision: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'vision' },
  vision: true,
}
const text: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'writer' },
  vision: false,
}
const textB: GenerationModelSelection = {
  ref: { providerId: 'openrouter', modelId: 'writer-b' },
  vision: false,
}

describe('generationPreconditions', () => {
  it.each([
    {
      name: 'no write model',
      images: [],
      observe: undefined,
      write: undefined,
      active: undefined,
      ok: false,
    },
    {
      name: 'photos and no observe model',
      images: [image],
      observe: undefined,
      write: text,
      active: undefined,
      ok: false,
    },
    {
      name: 'photos and a non-vision observe model',
      images: [image],
      observe: text,
      write: text,
      active: undefined,
      ok: false,
    },
    {
      name: 'no photos and no observe model',
      images: [],
      observe: undefined,
      write: text,
      active: undefined,
      ok: true,
    },
    {
      name: 'all required models',
      images: [image],
      observe: vision,
      write: text,
      active: undefined,
      ok: true,
    },
    {
      name: 'an active job',
      images: [],
      observe: undefined,
      write: text,
      active: { status: 'running' },
      ok: false,
    },
  ])('$name → $ok', ({ images, observe, write, active, ok }) => {
    expect(ordinaryGenerationPreconditions(images, observe, write, active).ok).toBe(ok)
  })

  it('does not require a comparison pair for ordinary generation', () => {
    expect(ordinaryGenerationPreconditions([], undefined, text, undefined).ok).toBe(true)
  })

  // spec/legacy/policy/generation.md: a deleted voice refuses every machine result, whatever the models.
  it('refuses a deleted voice before anything else', () => {
    const deleted = { deleted: true }
    expect(ordinaryGenerationPreconditions([], undefined, text, undefined, deleted)).toEqual({
      ok: false,
      reason: deletedVoiceAIReason(),
      // Not a setup blocker: no route to the models fixes a tombstoned voice, so the bar keeps
      // its disabled buttons and the reason under them.
      blocker: 'voiceDeleted',
    })
    expect(
      comparisonGenerationPreconditions([], undefined, text, textB, undefined, deleted).ok,
    ).toBe(false)
    expect(
      ordinaryGenerationPreconditions([], undefined, text, undefined, { deleted: false }).ok,
    ).toBe(true)
  })

  it('requires two distinct candidates only for A/B generation', () => {
    expect(comparisonGenerationPreconditions([], undefined, text, undefined, undefined).ok).toBe(
      false,
    )
    expect(comparisonGenerationPreconditions([], undefined, text, text, undefined).ok).toBe(false)
    expect(comparisonGenerationPreconditions([], undefined, text, textB, undefined).ok).toBe(true)
  })
})

describe('setup blockers', () => {
  // The editor drops its buttons and offers a route out only for the blockers a route can fix.
  it('routes the model blockers and leaves the rest to wait', () => {
    const missingWrite = ordinaryGenerationPreconditions([], undefined, undefined, undefined)
    expect(isSetupBlocker(missingWrite.blocker)).toBe(true)
    expect(setupBlockerTarget(missingWrite.blocker)).toBe('brief')

    // The A/B candidates moved into the brief with the active selections, so every fixable
    // blocker now names the one surface — nobody is sent to the AI 모델 page mid-draft.
    const missingPair = comparisonGenerationPreconditions([], undefined, text, undefined, undefined)
    expect(setupBlockerTarget(missingPair.blocker)).toBe('brief')
    expect(
      setupBlockerTarget(
        comparisonGenerationPreconditions([], undefined, text, text, undefined).blocker,
      ),
    ).toBe('brief')

    const missingObserve = ordinaryGenerationPreconditions([image], undefined, text, undefined)
    expect(setupBlockerTarget(missingObserve.blocker)).toBe('brief')

    const running = ordinaryGenerationPreconditions([], undefined, text, { status: 'running' })
    expect(running.blocker).toBe('activeJob')
    expect(isSetupBlocker(running.blocker)).toBe(false)
    expect(setupBlockerTarget(running.blocker)).toBeUndefined()
    expect(
      isSetupBlocker(
        ordinaryGenerationPreconditions([], undefined, text, undefined, { deleted: true }).blocker,
      ),
    ).toBe(false)

    // A run that CAN start carries no blocker at all, which is not a setup state either.
    expect(
      isSetupBlocker(ordinaryGenerationPreconditions([], undefined, text, undefined).blocker),
    ).toBe(false)
  })
})

// VIDEO-11: the video capability is checked per RUN, against the post's own clips — a
// video-blind model still serves every post without one.
describe('a post with a video', () => {
  const videoBlind = {
    ref: { providerId: 'p', modelId: 'observe' },
    vision: true,
    videoInput: false,
  }
  const watcher = {
    ref: { providerId: 'p', modelId: 'watcher' },
    vision: true,
    videoInput: true,
    signedVideoUrl: true,
  }
  const write = { ref: { providerId: 'p', modelId: 'write' }, vision: false }
  const image = { id: 'image-1' }
  const clip = { id: 'video-1' }

  it('refuses a video-blind observe model, and accepts one that can watch', () => {
    const blocked = ordinaryGenerationPreconditions(
      [image],
      videoBlind,
      write,
      undefined,
      undefined,
      [clip],
    )
    expect(blocked.ok).toBe(false)
    expect(blocked.blocker).toBe('videoModel')

    const allowed = ordinaryGenerationPreconditions([image], watcher, write, undefined, undefined, [
      clip,
    ])
    expect(allowed.ok).toBe(true)
  })

  it('leaves a post with no clip alone', () => {
    expect(
      ordinaryGenerationPreconditions([image], videoBlind, write, undefined, undefined, []).ok,
    ).toBe(true)
  })

  it('refuses inline-only models for generation and A/B while preserving raw video capability', () => {
    const inlineOnly = { ...watcher, signedVideoUrl: false, inlineStaticVideo: true }
    const secondWriter = { ...write, ref: { ...write.ref, modelId: 'other-writer' } }
    expect(
      ordinaryGenerationPreconditions([image], inlineOnly, write, undefined, undefined, [clip])
        .blocker,
    ).toBe('videoUrl')
    expect(
      comparisonGenerationPreconditions(
        [image],
        inlineOnly,
        write,
        secondWriter,
        undefined,
        undefined,
        [clip],
      ).blocker,
    ).toBe('videoUrl')
    expect(ordinaryGenerationPreconditions([image], inlineOnly, write, undefined).ok).toBe(true)
    expect(inlineOnly.videoInput).toBe(true)
    expect(inlineOnly.inlineStaticVideo).toBe(true)
  })

  // The simpler thing to fix comes first: a model that cannot see a photo is refused for that.
  it('reports the vision blocker first when the model can do neither', () => {
    const neither = { ref: videoBlind.ref, vision: false, videoInput: false }
    const result = ordinaryGenerationPreconditions([image], neither, write, undefined, undefined, [
      clip,
    ])
    expect(result.blocker).toBe('vision')
  })

  // The A/B path makes the same check: a comparison that cannot observe the clips would compare
  // two writers working from half the material.
  it('refuses the comparison too', () => {
    const result = comparisonGenerationPreconditions(
      [image],
      videoBlind,
      write,
      write,
      undefined,
      undefined,
      [clip],
    )
    expect(result.blocker).toBe('videoModel')
  })

  // A clip alone is still material: the run needs an observe model even with no photo.
  it('requires an observe model for a post that is only clips', () => {
    const result = ordinaryGenerationPreconditions([], undefined, write, undefined, undefined, [
      clip,
    ])
    expect(result.blocker).toBe('observe')
  })
})

// A published post takes no write at all (POST-86), so its refusal outranks every other one —
// a deleted voice and a running job included — and it is not a setup blocker: the brief cannot
// fix it, only clearing the post's URL on 글 완성 can.
describe('a published post', () => {
  const running = { status: 'running' }
  const deleted = { deleted: true }
  const locked = {
    ok: false,
    reason: '발행된 글은 바꿀 수 없어요. 글 완성에서 발행 URL을 지우면 다시 고칠 수 있어요.',
    blocker: 'published',
  }

  it('refuses an ordinary generation before the voice, the job or the models', () => {
    expect(
      ordinaryGenerationPreconditions([image], undefined, undefined, running, deleted, [], true),
    ).toEqual(locked)
    expect(
      ordinaryGenerationPreconditions([], undefined, text, undefined, undefined, [], true),
    ).toEqual(locked)
  })

  it('refuses the comparison the same way', () => {
    expect(
      comparisonGenerationPreconditions([image], undefined, text, text, running, deleted, [], true),
    ).toEqual(locked)
    expect(
      comparisonGenerationPreconditions([], undefined, text, textB, undefined, undefined, [], true),
    ).toEqual(locked)
  })

  it('is not a setup blocker', () => {
    const result = ordinaryGenerationPreconditions(
      [],
      undefined,
      text,
      undefined,
      undefined,
      [],
      true,
    )
    expect(isSetupBlocker(result.blocker)).toBe(false)
    expect(setupBlockerTarget(result.blocker)).toBeUndefined()
  })

  it('changes nothing for a post that is not published', () => {
    expect(
      ordinaryGenerationPreconditions([], undefined, text, undefined, undefined, [], false),
    ).toEqual({ ok: true, reason: '' })
    expect(
      comparisonGenerationPreconditions([], undefined, text, textB, running, undefined, [], false)
        .blocker,
    ).toBe('activeJob')
  })
})

// A refused press opens the brief with EVERY field its run is missing marked, in that field's own
// words, so one visit fixes them all.
describe('briefIssues', () => {
  it('marks every field 생성 is missing, and nothing of the pair', () => {
    expect(briefIssues('generation', 1, 0, undefined, undefined, undefined, undefined)).toEqual({
      observe: '관찰 모델을 선택하세요.',
      write: '활성 작성 모델을 선택하세요.',
    })
  })

  it('marks the pair for A/B 비교, and nothing of the active writer', () => {
    expect(briefIssues('comparison', 0, 0, undefined, undefined, text, undefined)).toEqual({
      pair: '작성 A/B 모델 두 개를 선택하세요.',
    })
    expect(briefIssues('comparison', 0, 0, undefined, undefined, text, text)).toEqual({
      pair: '서로 다른 작성 모델을 선택하세요.',
    })
  })

  it('asks nothing of the observe model for a post with no media', () => {
    expect(briefIssues('generation', 0, 0, undefined, text, undefined, undefined)).toEqual({})
  })

  it('marks an observe model that cannot see the photos', () => {
    expect(briefIssues('generation', 1, 0, text, text, undefined, undefined)).toEqual({
      observe: '사진을 볼 수 있는 관찰 모델을 선택하세요.',
    })
    expect(briefIssues('generation', 1, 0, vision, text, undefined, undefined)).toEqual({})
  })
})
