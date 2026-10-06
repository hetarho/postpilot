import { describe, expect, it } from 'vitest'
import { MediaPhaseRecorder } from './phase-metrics'

describe('media phase observations', () => {
  it('preserves unknown phases and does not add overlapping spans into elapsed time', () => {
    let now = 0
    const recorder = new MediaPhaseRecorder(['source', 'encode', 'upload'] as const, () => now)
    const source = recorder.begin('source')
    now = 2
    const encode = recorder.begin('encode')
    now = 8
    source()
    now = 10
    encode()
    const result = recorder.snapshot()
    expect(result.elapsedMs).toBe(10)
    expect(result.phases.source?.totalMs).toBe(8)
    expect(result.phases.encode?.totalMs).toBe(8)
    expect(result.phases.upload).toBeNull()
  })

  it('records failed work, aggregates observations and releases a span only once', async () => {
    let now = 0
    const recorder = new MediaPhaseRecorder(['decode'], () => now)
    expect(() =>
      recorder.measure('decode', () => {
        now += 3
        throw new Error('decode failed')
      }),
    ).toThrow('decode failed')
    await expect(
      recorder.measureAsync('decode', async () => {
        now += 5
        throw new Error('async failed')
      }),
    ).rejects.toThrow('async failed')
    const end = recorder.begin('decode')
    now += 2
    end()
    end()
    expect(recorder.snapshot().phases.decode).toEqual({
      samples: 3,
      totalMs: 10,
      minMs: 2,
      maxMs: 5,
    })
  })

  it('returns isolated snapshots and rejects invalid phase or clock observations', () => {
    let now = 0
    const recorder = new MediaPhaseRecorder<string>(['decode'], () => now)
    recorder.measure('decode', () => (now = 1))
    recorder.snapshot().phases.decode!.totalMs = 999
    expect(recorder.snapshot().phases.decode?.totalMs).toBe(1)
    expect(() => new MediaPhaseRecorder(['decode', 'decode'])).toThrow()
    expect(() => recorder.begin('other')).toThrow('Unknown')
    const end = recorder.begin('decode')
    now = -1
    expect(end).toThrow('clock')
  })
})
