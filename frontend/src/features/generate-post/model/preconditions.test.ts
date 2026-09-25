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
    expect(
      ordinaryGenerationPreconditions({
        images,
        videos: [],
        published: false,
        activeJob: active,
        voice: undefined,
        observe,
        write,
      }).ok,
    ).toBe(ok)
  })

  it('does not require a comparison pair for ordinary generation', () => {
    expect(
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        write: text,
      }).ok,
    ).toBe(true)
  })

  // spec/legacy/policy/generation.md: a deleted voice refuses every machine result, whatever the models.
  it('refuses a deleted voice before anything else', () => {
    const deleted = { deleted: true }
    expect(
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: deleted,
        observe: undefined,
        write: text,
      }),
    ).toEqual({
      ok: false,
      reason: deletedVoiceAIReason(),
      // Not a setup blocker: no route to the models fixes a tombstoned voice, so the bar keeps
      // its disabled buttons and the reason under them.
      blocker: 'voiceDeleted',
    })
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: deleted,
        observe: undefined,
        writeA: text,
        writeB: textB,
      }).ok,
    ).toBe(false)
    expect(
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: { deleted: false },
        observe: undefined,
        write: text,
      }).ok,
    ).toBe(true)
  })

  it('requires two distinct candidates only for A/B generation', () => {
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: undefined,
      }).ok,
    ).toBe(false)
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: text,
      }).ok,
    ).toBe(false)
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: textB,
      }).ok,
    ).toBe(true)
  })
})

describe('setup blockers', () => {
  // The editor drops its buttons and offers a route out only for the blockers a route can fix.
  it('routes the model blockers and leaves the rest to wait', () => {
    const missingWrite = ordinaryGenerationPreconditions({
      images: [],
      videos: [],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: undefined,
      write: undefined,
    })
    expect(isSetupBlocker(missingWrite.blocker)).toBe(true)
    expect(setupBlockerTarget(missingWrite.blocker)).toBe('brief')

    // The A/B candidates moved into the brief with the active selections, so every fixable
    // blocker now names the one surface — nobody is sent to the AI 모델 page mid-draft.
    const missingPair = comparisonGenerationPreconditions({
      images: [],
      videos: [],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: undefined,
      writeA: text,
      writeB: undefined,
    })
    expect(setupBlockerTarget(missingPair.blocker)).toBe('brief')
    expect(
      setupBlockerTarget(
        comparisonGenerationPreconditions({
          images: [],
          videos: [],
          published: false,
          activeJob: undefined,
          voice: undefined,
          observe: undefined,
          writeA: text,
          writeB: text,
        }).blocker,
      ),
    ).toBe('brief')

    const missingObserve = ordinaryGenerationPreconditions({
      images: [image],
      videos: [],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: undefined,
      write: text,
    })
    expect(setupBlockerTarget(missingObserve.blocker)).toBe('brief')

    const running = ordinaryGenerationPreconditions({
      images: [],
      videos: [],
      published: false,
      activeJob: { status: 'running' },
      voice: undefined,
      observe: undefined,
      write: text,
    })
    expect(running.blocker).toBe('activeJob')
    expect(isSetupBlocker(running.blocker)).toBe(false)
    expect(setupBlockerTarget(running.blocker)).toBeUndefined()
    expect(
      isSetupBlocker(
        ordinaryGenerationPreconditions({
          images: [],
          videos: [],
          published: false,
          activeJob: undefined,
          voice: { deleted: true },
          observe: undefined,
          write: text,
        }).blocker,
      ),
    ).toBe(false)

    // A run that CAN start carries no blocker at all, which is not a setup state either.
    expect(
      isSetupBlocker(
        ordinaryGenerationPreconditions({
          images: [],
          videos: [],
          published: false,
          activeJob: undefined,
          voice: undefined,
          observe: undefined,
          write: text,
        }).blocker,
      ),
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
    const blocked = ordinaryGenerationPreconditions({
      images: [image],
      videos: [clip],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: videoBlind,
      write,
    })
    expect(blocked.ok).toBe(false)
    expect(blocked.blocker).toBe('videoModel')

    const allowed = ordinaryGenerationPreconditions({
      images: [image],
      videos: [clip],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: watcher,
      write,
    })
    expect(allowed.ok).toBe(true)
  })

  it('leaves a post with no clip alone', () => {
    expect(
      ordinaryGenerationPreconditions({
        images: [image],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: videoBlind,
        write,
      }).ok,
    ).toBe(true)
  })

  it('refuses inline-only models for generation and A/B while preserving raw video capability', () => {
    const inlineOnly = { ...watcher, signedVideoUrl: false, inlineStaticVideo: true }
    const secondWriter = { ...write, ref: { ...write.ref, modelId: 'other-writer' } }
    expect(
      ordinaryGenerationPreconditions({
        images: [image],
        videos: [clip],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: inlineOnly,
        write,
      }).blocker,
    ).toBe('videoUrl')
    expect(
      comparisonGenerationPreconditions({
        images: [image],
        videos: [clip],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: inlineOnly,
        writeA: write,
        writeB: secondWriter,
      }).blocker,
    ).toBe('videoUrl')
    expect(
      ordinaryGenerationPreconditions({
        images: [image],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: inlineOnly,
        write,
      }).ok,
    ).toBe(true)
    expect(inlineOnly.videoInput).toBe(true)
    expect(inlineOnly.inlineStaticVideo).toBe(true)
  })

  // The simpler thing to fix comes first: a model that cannot see a photo is refused for that.
  it('reports the vision blocker first when the model can do neither', () => {
    const neither = { ref: videoBlind.ref, vision: false, videoInput: false }
    const result = ordinaryGenerationPreconditions({
      images: [image],
      videos: [clip],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: neither,
      write,
    })
    expect(result.blocker).toBe('vision')
  })

  // The A/B path makes the same check: a comparison that cannot observe the clips would compare
  // two writers working from half the material.
  it('refuses the comparison too', () => {
    const result = comparisonGenerationPreconditions({
      images: [image],
      videos: [clip],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: videoBlind,
      writeA: write,
      writeB: write,
    })
    expect(result.blocker).toBe('videoModel')
  })

  // A clip alone is still material: the run needs an observe model even with no photo.
  it('requires an observe model for a post that is only clips', () => {
    const result = ordinaryGenerationPreconditions({
      images: [],
      videos: [clip],
      published: false,
      activeJob: undefined,
      voice: undefined,
      observe: undefined,
      write,
    })
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
      ordinaryGenerationPreconditions({
        images: [image],
        videos: [],
        published: true,
        activeJob: running,
        voice: deleted,
        observe: undefined,
        write: undefined,
      }),
    ).toEqual(locked)
    expect(
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        published: true,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        write: text,
      }),
    ).toEqual(locked)
  })

  it('refuses the comparison the same way', () => {
    expect(
      comparisonGenerationPreconditions({
        images: [image],
        videos: [],
        published: true,
        activeJob: running,
        voice: deleted,
        observe: undefined,
        writeA: text,
        writeB: text,
      }),
    ).toEqual(locked)
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: true,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: textB,
      }),
    ).toEqual(locked)
  })

  it('is not a setup blocker', () => {
    const result = ordinaryGenerationPreconditions({
      images: [],
      videos: [],
      published: true,
      activeJob: undefined,
      voice: undefined,
      observe: undefined,
      write: text,
    })
    expect(isSetupBlocker(result.blocker)).toBe(false)
    expect(setupBlockerTarget(result.blocker)).toBeUndefined()
  })

  it('changes nothing for a post that is not published', () => {
    expect(
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        write: text,
      }),
    ).toEqual({ ok: true, reason: '' })
    expect(
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        published: false,
        activeJob: running,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: textB,
      }).blocker,
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

// Review F23: every member of the gate's input is required, so a caller that forgets one — the
// lab once forgot `published` — fails to compile rather than passing the blocker it feeds. These
// only typecheck; `tsc -b` in the build is what runs them.
describe('the gate input', () => {
  it('requires every member, published included', () => {
    const omitsPublished = () =>
      // @ts-expect-error `published` is required.
      ordinaryGenerationPreconditions({
        images: [],
        videos: [],
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        write: text,
      })
    const comparisonOmitsPublished = () =>
      // @ts-expect-error `published` is required.
      comparisonGenerationPreconditions({
        images: [],
        videos: [],
        activeJob: undefined,
        voice: undefined,
        observe: undefined,
        writeA: text,
        writeB: textB,
      })
    void omitsPublished
    void comparisonOmitsPublished
  })
})
