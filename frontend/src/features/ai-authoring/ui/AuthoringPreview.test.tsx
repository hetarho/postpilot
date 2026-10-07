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
it('opens restored unfinished video source with its valid selection and keeps the latest valid direct preview', () => {
  const previous = artifact(CLIP_COMPOSITION_EXAMPLE)
  const changed = CLIP_COMPOSITION_EXAMPLE.replace('가게 앞', '산책 입구').replace(
    '간판과 외관',
    '입구 풍경',
  )
  const { rerender, container } = render(
    <AuthoringPreview
      kind="video-template"
      artifact={artifact('<clip version="1"><stage')}
      fallbackArtifact={previous}
    />,
  )
  expect(screen.getByText('가게 앞')).toBeInTheDocument()
  expect(screen.getByText('간판과 외관')).toBeInTheDocument()
  rerender(
    <AuthoringPreview
      kind="video-template"
      artifact={artifact(changed)}
      fallbackArtifact={previous}
    />,
  )
  expect(screen.getByText('산책 입구')).toBeInTheDocument()
  expect(screen.getByText('입구 풍경')).toBeInTheDocument()
  expect(screen.queryByText('가게 앞')).toBeNull()
  rerender(
    <AuthoringPreview
      kind="video-template"
      artifact={artifact('<clip>unfinished latest')}
      fallbackArtifact={previous}
    />,
  )
  expect(screen.getByText('산책 입구')).toBeInTheDocument()
  expect(screen.getByText('입구 풍경')).toBeInTheDocument()
  expect(screen.queryByText('가게 앞')).toBeNull()
  expect(container).not.toHaveTextContent('<clip>')
  expect(container).not.toHaveTextContent('unfinished latest')
})
it('retains a valid video preview while newly typed source becomes invalid without a saved fallback', () => {
  const { rerender } = render(
    <AuthoringPreview kind="video-template" artifact={artifact(CLIP_COMPOSITION_EXAMPLE)} />,
  )
  rerender(<AuthoringPreview kind="video-template" artifact={artifact('<clip version="1">')} />)
  expect(screen.getByText('가게 앞')).toBeInTheDocument()
  expect(screen.getByText('첫 한 입의 인상')).toBeInTheDocument()
})
it('shows an unavailable preview when current and fallback video source are both invalid', () => {
  const { container } = render(
    <AuthoringPreview
      kind="video-template"
      artifact={artifact('<clip>unfinished')}
      fallbackArtifact={artifact('<clip>also unfinished')}
    />,
  )
  expect(
    screen.getByText('읽을 수 있는 초안을 준비하지 못했어요. 이전 제안으로 다시 준비해 주세요.'),
  ).toBeInTheDocument()
  expect(screen.queryByRole('list')).toBeNull()
  expect(container).not.toHaveTextContent('<clip>')
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

it('opens restored invalid template input with its last valid preview and updates that preview on valid direct changes', () => {
  const last = artifact('확인한 본문')
  last.titleArea = '확인한 제목'
  const { rerender } = render(
    <AuthoringPreview
      kind="post-template"
      artifact={{ ...artifact('<write>unfinished'), titleArea: '<write>unfinished title' }}
      fallbackArtifact={last}
    />,
  )
  expect(screen.getByText('확인한 본문')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '확인한 제목' })).toBeInTheDocument()
  rerender(
    <AuthoringPreview
      kind="post-template"
      artifact={artifact('직접 수정한 본문')}
      fallbackArtifact={last}
    />,
  )
  expect(screen.getByText('직접 수정한 본문')).toBeInTheDocument()
  rerender(
    <AuthoringPreview
      kind="post-template"
      artifact={artifact('<write>still unfinished')}
      fallbackArtifact={last}
    />,
  )
  expect(screen.getByText('직접 수정한 본문')).toBeInTheDocument()
})
