import { describe, expect, it } from 'vitest'
import type { ClipObservedSegment } from '@/entities/clip-observation'
import { observationEditOf, observationId } from './observation-edits'

const segment = (speech: string): ClipObservedSegment => ({
  startMs: 0,
  endMs: 1000,
  event: '장면',
  action: '동작',
  motion: '',
  subjects: ['음식'],
  speech,
  quality: '',
  certainty: 'certain',
  usability: 'usable',
})

describe('observation corrections (CLIP-235)', () => {
  it('keys a scene by its source and position', () => {
    expect(observationId('src', 3)).toBe('src/3')
  })

  it('reads subjects one per line, dropping blank lines', () => {
    expect(observationEditOf('s/0', 'subjects', segment(''), ' 국수 \n\n 젓가락')).toEqual({
      observationId: 's/0',
      subjects: ['국수', '젓가락'],
    })
  })

  it('refuses clearing observed speech and saves other text as typed', () => {
    expect(observationEditOf('s/0', 'speech', segment('맛있어요'), '  ')).toBeUndefined()
    expect(observationEditOf('s/0', 'speech', segment('맛있어요'), '맛있다')).toEqual({
      observationId: 's/0',
      speech: '맛있다',
    })
    expect(observationEditOf('s/0', 'event', segment(''), '')).toEqual({
      observationId: 's/0',
      event: '',
    })
  })
})
