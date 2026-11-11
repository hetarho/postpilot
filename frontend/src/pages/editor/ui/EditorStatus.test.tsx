import { render, screen } from '@testing-library/react'
import type { GenerationJob } from '@/entities/generation-job'
import { EditorStatusLine } from './EditorStatus'

const runningJob: GenerationJob = {
  id: 'job-1',
  kind: 'generate',
  status: 'running',
  stage: 'write',
  progressDone: 0,
  progressTotal: 1,
  failure: undefined,
  postSlug: 'post-1',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '2026-11-11T00:00:00Z',
  updatedAt: '2026-11-11T00:00:00Z',
  targetLanguage: undefined,
}

describe('EditorStatusLine', () => {
  // The routine words sat on their own line above 요청 기술 보기 and read like a log printed on
  // the page; they are kept for a screen reader only.
  it.each(['idle', 'saving', 'saved'] as const)(
    'keeps the routine %s state off the screen but in the live region',
    (saveState) => {
      render(
        <EditorStatusLine
          job={undefined}
          saveState={saveState}
          status="draft"
          className="order-last w-full"
        />,
      )
      const line = screen.getByRole('status')
      expect(line).toHaveClass('sr-only')
      expect(line).not.toHaveClass('w-full')
      expect(line).toHaveAttribute('aria-live', 'polite')
    },
  )

  it('shows a failing save on the row, in the danger colour', () => {
    render(<EditorStatusLine job={undefined} saveState="error" status="draft" className="w-full" />)
    const line = screen.getByRole('status')
    expect(line).not.toHaveClass('sr-only')
    expect(line).toHaveClass('text-notice-danger-fg', 'w-full')
    expect(line).not.toBeEmptyDOMElement()
  })

  it('shows the stage of a running job', () => {
    render(<EditorStatusLine job={runningJob} saveState="saved" status="draft" />)
    const line = screen.getByRole('status')
    expect(line).not.toHaveClass('sr-only')
    expect(line).not.toBeEmptyDOMElement()
  })
})
