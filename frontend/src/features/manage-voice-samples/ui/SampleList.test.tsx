import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { SampleList } from './SampleList'

afterEach(() => vi.restoreAllMocks())

describe('SampleList', () => {
  it('deletes after confirmation and hands the re-analysis job to its page', async () => {
    const calls: string[] = []
    const samples = [
      { id: 'sample-1', label: '제주', chars: 240, createdAt: '2026-08-29T12:00:00Z' },
      { id: 'sample-2', label: '서울', chars: 260, createdAt: '2026-08-28T12:00:00Z' },
    ]
    const transport = createFakeAuthTransport({
      calls,
      voice: { samples, deleteJobId: 'reanalyze-1' },
    })
    const onAnalysisStarted = vi.fn()
    render(<SampleList ownerId="alice" samples={samples} onAnalysisStarted={onAnalysisStarted} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })

    await userEvent.click(screen.getByRole('button', { name: '제주 삭제' }))
    // The confirmation is the Dialog sheet, not window.confirm — its confirm button is the only
    // control named exactly 삭제 (the rows are '<라벨> 삭제').
    await userEvent.click(await screen.findByRole('button', { name: '삭제' }))

    await waitFor(() => expect(calls).toContain('DeleteVoiceSample'))
    expect(onAnalysisStarted).toHaveBeenCalledWith('reanalyze-1')
  })
})
