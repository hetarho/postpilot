import { describe, expect, it } from 'vitest'
import { LEVELS, isLevelName, levelOf, orderModelsForStage } from './level'
import type { CatalogModel, StageName } from './types'

const model = (
  modelId: string,
  stages: StageName[],
  levels: CatalogModel['levels'] = {},
): CatalogModel => ({
  ref: { providerId: 'openrouter', modelId },
  label: modelId,
  vision: stages.includes('observe'),
  videoInput: false,
  structuredOutput: false,
  stages,
  levels,
  disabled: false,
  disabledReason: '',
  contextTokens: 0n,
  inputUsdPerMillion: '',
  outputUsdPerMillion: '',
  pricingCheckedAt: '',
  requiredCredits: 5,
  affordable: true,
})

describe('orderModelsForStage', () => {
  it('orders 가성비 → 최고 (MODEL-44)', () => {
    const models = [
      model('top', ['write'], { write: 'top' }),
      model('value', ['write'], { write: 'value' }),
      model('premium', ['write'], { write: 'premium' }),
      model('balanced', ['write'], { write: 'balanced' }),
    ]
    expect(orderModelsForStage(models, 'write').map((m) => m.ref.modelId)).toEqual([
      'value',
      'balanced',
      'premium',
      'top',
    ])
  })

  it('puts every ungraded model after every graded one (MODEL-58)', () => {
    const models = [
      model('ungraded-a', ['write']),
      model('top', ['write'], { write: 'top' }),
      model('ungraded-b', ['write']),
      model('value', ['write'], { write: 'value' }),
    ]
    expect(orderModelsForStage(models, 'write').map((m) => m.ref.modelId)).toEqual([
      'value',
      'top',
      // The operator's backlog, in the order the server sent it.
      'ungraded-a',
      'ungraded-b',
    ])
  })

  it('is stable, so the server order survives inside one grade', () => {
    const models = [
      model('second', ['write'], { write: 'value' }),
      model('first', ['write'], { write: 'value' }),
      model('third', ['write'], { write: 'value' }),
    ]
    expect(orderModelsForStage(models, 'write').map((m) => m.ref.modelId)).toEqual([
      'second',
      'first',
      'third',
    ])
  })

  it('reads the asked-for stage only — a grade is per stage, not per model', () => {
    // The whole reason the column sits on the registration: photo analysis pays for input
    // tokens per photo and writing pays for output, so one model is two different bargains.
    const models = [
      model('a', ['observe', 'write'], { observe: 'top', write: 'value' }),
      model('b', ['observe', 'write'], { observe: 'value', write: 'top' }),
    ]
    expect(orderModelsForStage(models, 'observe').map((m) => m.ref.modelId)).toEqual(['b', 'a'])
    expect(orderModelsForStage(models, 'write').map((m) => m.ref.modelId)).toEqual(['a', 'b'])
  })

  it('does not mutate its input', () => {
    const models = [
      model('top', ['write'], { write: 'top' }),
      model('value', ['write'], { write: 'value' }),
    ]
    orderModelsForStage(models, 'write')
    expect(models.map((m) => m.ref.modelId)).toEqual(['top', 'value'])
  })
})

describe('levelOf / isLevelName', () => {
  it('reports the stage grade, or undefined when there is none', () => {
    const m = model('a', ['observe', 'write'], { observe: 'premium' })
    expect(levelOf(m, 'observe')).toBe('premium')
    expect(levelOf(m, 'write')).toBeUndefined()
  })

  it('refuses a grade this build does not know, so unknown copy is never rendered', () => {
    expect(LEVELS.every(isLevelName)).toBe(true)
    expect(isLevelName('legendary')).toBe(false)
    expect(isLevelName('')).toBe(false)
  })
})
