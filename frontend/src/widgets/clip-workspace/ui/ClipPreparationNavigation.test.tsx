import { act, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { expect, it } from 'vitest'
import { Button } from '@/shared/ui'
import { clipGuideTo } from '@/test/clip-preparation'

function LoadingWorkspace({ ready }: { ready: boolean }) {
  const [current, select] = useState('소스')
  return (
    <>
      <a href="/naver-clip">네이버 클립으로 돌아가기</a>
      {ready && (
        <nav aria-label="클립 만들기 단계">
          {['소스', '이야기', '스타일', '확인'].map((name) => (
            <Button
              key={name}
              aria-current={current === name ? 'step' : undefined}
              onClick={() => select(name)}
            >
              {name}
            </Button>
          ))}
        </nav>
      )}
    </>
  )
}
it('waits past loading chrome before selecting a real visible preparation screen', async () => {
  const view = render(<LoadingWorkspace ready={false} />)
  const movement = clipGuideTo('style')
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 10))
  })
  view.rerender(<LoadingWorkspace ready />)
  await movement
  expect(screen.getByRole('button', { name: '스타일' })).toHaveAttribute('aria-current', 'step')
})
