import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CLIP_ESTIMATE_STORAGE_KEY, PLAN_ESTIMATE_STORAGE_KEY } from '../config'
import { useClipEstimateInput, useEstimateInput } from './estimate-input'

beforeEach(() => localStorage.clear())

describe('stored plan conditions', () => {
  it('bounds each independent preference and rejects non-numbers', () => {
    localStorage.setItem(
      PLAN_ESTIMATE_STORAGE_KEY,
      JSON.stringify({ chars: 99999, photos: -5, videos: '2' }),
    )
    localStorage.setItem(CLIP_ESTIMATE_STORAGE_KEY, JSON.stringify({ sources: 99, seconds: 0 }))
    expect(renderHook(useEstimateInput).result.current[0]).toEqual({
      chars: 5000,
      photos: 0,
      videos: 0,
    })
    expect(renderHook(useClipEstimateInput).result.current[0]).toEqual({ sources: 20, seconds: 15 })
  })
  it.each(['null', '[]', 'unreadable'])('defaults malformed storage: %s', (value) => {
    localStorage.setItem(CLIP_ESTIMATE_STORAGE_KEY, value)
    expect(renderHook(useClipEstimateInput).result.current[0]).toEqual({ sources: 3, seconds: 30 })
  })
  it('remains interactive when browser storage is unavailable', () => {
    const read = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    const write = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied')
    })
    try {
      const { result } = renderHook(useClipEstimateInput)
      act(() => result.current[1]({ sources: 4, seconds: 45 }))
      expect(result.current[0]).toEqual({ sources: 4, seconds: 45 })
    } finally {
      read.mockRestore()
      write.mockRestore()
    }
  })
})
