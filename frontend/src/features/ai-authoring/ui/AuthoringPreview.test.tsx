import { beforeEach, afterEach, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'
import type { AuthoringArtifact, AuthoringKind } from '@/entities/ai-authoring'
import { CLIP_COMPOSITION_EXAMPLE } from '@/entities/clip-template'
import { AuthoringPreview } from './AuthoringPreview'

beforeEach(() => initializeI18n('ko'))
afterEach(cleanup)
const artifact = (body: string): AuthoringArtifact => ({
  id: 'a',
  name: '추천',
  description: '읽기 쉬운 설명',
  body,
  titleArea: '',
})
it('renders the actual blog structure locally without exposing template source', () => {
  const body = '<write>오늘의 경험</write>\n<slot kind="photo" count="2"/>\n마지막 인사'
  const { container } = render(<AuthoringPreview kind="post-template" artifact={artifact(body)} />)
  expect(screen.getByText('오늘의 경험')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '사진 2장' })).toBeInTheDocument()
  expect(screen.getByText('마지막 인사')).toBeInTheDocument()
  expect(container).not.toHaveTextContent('<write>')
})
it('shows actual video stages and plain caption entries without interactive specialist controls', () => {
  const { container } = render(
    <AuthoringPreview kind="video-template" artifact={artifact(CLIP_COMPOSITION_EXAMPLE)} />,
  )
  expect(screen.getByText('가게 앞')).toBeInTheDocument()
  expect(screen.getByText('간판과 외관')).toBeInTheDocument()
  expect(screen.getByText('첫 한 입의 인상')).toBeInTheDocument()
  expect(container).not.toHaveTextContent('<clip')
  expect(screen.queryByRole('slider')).toBeNull()
  expect(screen.queryByRole('combobox')).toBeNull()
})
it.each(['post-guideline', 'video-guideline', 'writing-voice'] satisfies AuthoringKind[])(
  'keeps %s readable and refuses unexpected compiled payloads',
  (kind) => {
    const { rerender, container } = render(
      <AuthoringPreview kind={kind} artifact={artifact('개인 경험을 편안하게 이야기해 주세요.')} />,
    )
    expect(screen.getByText('개인 경험을 편안하게 이야기해 주세요.')).toBeInTheDocument()
    if (kind === 'writing-voice') expect(screen.getByText(/가상의 예문/)).toBeInTheDocument()
    rerender(<AuthoringPreview kind={kind} artifact={artifact('{"body":"hidden"}')} />)
    expect(container).not.toHaveTextContent('hidden')
    rerender(<AuthoringPreview kind={kind} artifact={artifact('<write>hidden</write>')} />)
    expect(container).not.toHaveTextContent('<write>')
    expect(container).not.toHaveTextContent('hidden')
  },
)
