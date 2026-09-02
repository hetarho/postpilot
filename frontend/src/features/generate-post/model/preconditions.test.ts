import { describe, expect, it } from 'vitest'
import { deletedVoiceAIReason } from '@/entities/voice'
import {
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

  // spec/policy/generation.md: a deleted voice refuses every machine result, whatever the models.
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
