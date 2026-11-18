import { expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { ProtoConfigurationKind } from '@/shared/api'
import { renderAppAt } from '@/test/app'

// 영상 템플릿 is 템플릿's directory (THEME-76, CLIP-42): unsaved conversations first as resume rows,
// then each saved template as one tile leading with its static 9:16 example, its name, its stages
// in order, how many videos use it and at most one work state — and no delete.
it('draws resume rows before the saved tiles, each tile one link with its miniature', async () => {
  renderAppAt('/video-templates', {
    user: { id: 'alice' },
    clips: {
      templates: [
        {
          id: 'visual',
          name: '나만의 영상 구성',
          compositionBody:
            '<clip version="1"><stage name="첫인상">첫 장면</stage><stage name="대표 메뉴">메뉴</stage></clip>',
          introPreset: 'cover',
          outroPreset: 'e',
          allowedCaptionStyles: ['neon'],
          projectCount: 3,
        },
      ],
    },
    authoring: {
      summaries: [
        {
          sessionId: 'draft-session',
          kind: ProtoConfigurationKind.VIDEO_TEMPLATE,
          targetId: '',
          displayName: '아직 만드는 영상',
          revision: 1,
          hasUnpublishedChanges: true,
          updatedAt: '2026-11-19T00:00:00Z',
        },
      ],
    },
  })
  const resume = await screen.findByRole('region', { name: '이어서 만들던 대화' })
  const directory = await screen.findByRole('region', { name: '저장된 영상 템플릿' })
  // The resume rows come first, apart from the saved tiles.
  expect(resume.compareDocumentPosition(directory) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(
    within(resume).getByRole('link', { name: '영상 템플릿 “아직 만드는 영상” 이어서 만들기' }),
  ).toHaveAttribute('href', '/video-templates/new?session=draft-session')

  const tiles = within(directory).getAllByRole('listitem')
  expect(tiles).toHaveLength(1)
  const link = within(tiles[0]).getByRole('link')
  expect(link).toHaveAttribute('href', '/video-templates/visual')
  // The miniature is decorative beside the name, so the link reads as the template.
  expect(link).toHaveAccessibleName(/^나만의 영상 구성/)
  expect(within(link).getByText('구성 예시')).toBeInTheDocument()
  expect(within(link).getByText('첫인상 · 대표 메뉴')).toBeInTheDocument()
  expect(within(link).getByText('영상 3개')).toBeInTheDocument()
  expect(within(tiles[0]).queryByRole('button')).toBeNull()
  // The visible heading and the saved-and-usable boilerplate are gone; the region keeps its name.
  expect(screen.getByRole('heading', { name: '저장된 영상 템플릿' })).toHaveClass('sr-only')
  expect(screen.queryByText('영상에 사용할 수 있어요')).toBeNull()
  expect(screen.queryByText(/저장하지 않은 영상 템플릿 작업을 확인/)).toBeNull()
})

// A conversation still busy says so on its resume row, which still reopens it (THEME-76).
it('keeps an unsaved conversation’s work state on its resume row', async () => {
  renderAppAt('/video-templates', {
    user: { id: 'alice' },
    clips: { templates: [] },
    authoring: {
      summaries: [
        {
          sessionId: 'busy-session',
          kind: ProtoConfigurationKind.VIDEO_TEMPLATE,
          targetId: '',
          displayName: '만드는 중',
          revision: 2,
          hasUnpublishedChanges: true,
          activeJobId: 'job-1',
          updatedAt: '2026-11-19T00:00:00Z',
        },
        {
          sessionId: 'saving-session',
          kind: ProtoConfigurationKind.VIDEO_TEMPLATE,
          targetId: '',
          displayName: '저장 중',
          revision: 3,
          hasUnpublishedChanges: true,
          publicationPending: true,
          updatedAt: '2026-11-19T00:00:00Z',
        },
        {
          sessionId: 'quiet-session',
          kind: ProtoConfigurationKind.VIDEO_TEMPLATE,
          targetId: '',
          displayName: '조용한 대화',
          revision: 1,
          hasUnpublishedChanges: true,
          updatedAt: '2026-11-19T00:00:00Z',
        },
      ],
    },
  })
  const resume = await screen.findByRole('region', { name: '이어서 만들던 대화' })
  const busy = within(resume).getByRole('link', { name: /“만드는 중” 이어서 만들기/ })
  expect(busy).toHaveAttribute('href', '/video-templates/new?session=busy-session')
  expect(within(busy).getByRole('status')).toHaveTextContent(
    '영상 템플릿 “만드는 중”의 AI 작업이 진행 중이에요.',
  )
  const saving = within(resume).getByRole('link', { name: /“저장 중” 이어서 만들기/ })
  expect(saving).toHaveAttribute('href', '/video-templates/new?session=saving-session')
  expect(within(saving).getByRole('status')).toHaveTextContent(
    '영상 템플릿 “저장 중”의 저장 결과를 확인해야 해요.',
  )
  // Being unsaved is what the region says; a quiet conversation repeats nothing.
  const quiet = within(resume).getByRole('link', {
    name: '영상 템플릿 “조용한 대화” 이어서 만들기',
  })
  expect(within(quiet).queryByRole('status')).toBeNull()
})
