import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import {
  WritingTestCandidateStatus,
  WritingTestPublicationAction,
  WritingTestStatus,
} from '@/shared/api'
import { writingTestI18n } from '@/features/writing-test'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { wireObservationTest } from '@/entities/writing-test/api/fixtures.test-support'
import { createTestQueryClient, withProviders } from '@/test/session'
import { WritingTestStudio } from './WritingTestStudio'

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
  i18next.addResourceBundle('en', 'writingTests', writingTestI18n.en, true, true)
})
afterEach(cleanup)

/** The detail page (/tests/$testId) over a stored photo observation test. */
function open(fixture: ReturnType<typeof createWritingTestStudioFixture>) {
  return render(
    <WritingTestStudio
      ownerId="alice"
      seedKey="test:test"
      testId="test"
      initialPlan={fixture.plan}
    />,
    { wrapper: withProviders(fixture.transport, createTestQueryClient()) },
  )
}

it('compares photo by photo, reveals the models, and adopts the champion as the observe AI only on explicit action', async () => {
  const fixture = createWritingTestStudioFixture()
  fixture.restoreTest(wireObservationTest(['p1', 'p2'], 2))
  open(fixture)
  expect(
    await screen.findByRole('heading', { level: 2, name: '어떤 관찰이 더 정확한가요?' }),
  ).toBeVisible()
  expect(screen.getAllByRole('img').map((image) => image.getAttribute('src'))).toEqual([
    'https://storage.test/p1',
    'https://storage.test/p2',
  ])
  expect(screen.getByRole('region', { name: '후보 B · p2.jpg' })).toHaveTextContent('Scene 1')
  expect(screen.queryByText(/model-0|model-1|Frozen setting/)).not.toBeInTheDocument()
  expect(fixture.votes).toHaveLength(0)

  await userEvent.click(screen.getByRole('button', { name: '후보 B를 승자로 선택' }))
  await waitFor(() => expect(fixture.votes).toHaveLength(1))
  expect(fixture.votes[0]).toMatchObject({ testId: 'test', winnerCandidateId: 'candidate-1' })

  expect(
    await screen.findByRole('heading', { level: 2, name: '마음에 드는 사진 보는 AI를 찾았어요' }),
  ).toBeVisible()
  expect(screen.getAllByText(/Frozen setting 2/).length).toBeGreaterThan(0)
  const result = screen.getByRole('region', { name: '우승 관찰 결과' })
  expect(within(result).getAllByRole('img')).toHaveLength(2)
  expect(result).toHaveTextContent('Scene 1')
  // A photo test's champion never reaches a post: no application, copy or post-model wording.
  expect(
    screen.queryByRole('button', {
      name: /원본 글에 결과 적용|활성 모델로 변경하기|네이버용 글 복사/,
    }),
  ).not.toBeInTheDocument()
  expect(screen.queryByText('우승 글 결과')).not.toBeInTheDocument()
  expect(fixture.applications).toHaveLength(0)
  expect(fixture.publications).toHaveLength(0)

  await userEvent.click(screen.getByRole('button', { name: '사진 보는 AI로 쓰기' }))
  expect(fixture.publications).toHaveLength(0)
  await userEvent.click(await screen.findByRole('button', { name: '사진 보는 AI로 쓰기' }))
  await waitFor(() => expect(fixture.publications).toHaveLength(1))
  expect(fixture.publications[0]).toMatchObject({
    action: WritingTestPublicationAction.ADOPT_MODEL,
    winnerCandidateId: 'candidate-1',
    makeDefault: false,
  })
  expect(await screen.findByText('“Frozen setting 2”를 사진 보는 AI로 쓰고 있어요')).toBeVisible()
  expect(fixture.applications).toHaveLength(0)
  expect(fixture.admissions).toHaveLength(0)
})

it('reads a purged photo test as retained metadata under the expired notice, with no photos, observations or adoption', async () => {
  const fixture = createWritingTestStudioFixture()
  const stored = wireObservationTest(['p1', 'p2'], 2, true)
  stored.contentExpiresAt = '2020-01-01T00:00:00Z'
  stored.candidates.forEach((candidate) => (candidate.observations = []))
  fixture.restoreTest(stored)
  open(fixture)
  expect(
    await screen.findByRole('heading', {
      level: 2,
      name: '사진과 관찰 결과의 보관 기간이 지났어요',
    }),
  ).toBeVisible()
  const result = await screen.findByRole('region', { name: '우승 관찰 결과' })
  expect(within(result).getByRole('status')).toHaveTextContent(
    '사진과 관찰 결과의 보관 기간이 지났어요',
  )
  expect(result).toHaveTextContent('비교한 사진 2장')
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.queryByText(/Scene/)).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /승자로 선택|사진 보는 AI로 쓰기/ }),
  ).not.toBeInTheDocument()
  expect(fixture.publications).toHaveLength(0)
})

it('keeps failed-only retry and abandonment for a photo test that only partly observed', async () => {
  const fixture = createWritingTestStudioFixture()
  const stored = wireObservationTest(['p1', 'p2'], 4)
  stored.status = WritingTestStatus.PARTIAL
  stored.candidates[3]!.status = WritingTestCandidateStatus.FAILED
  stored.candidates[3]!.observations = []
  stored.matches = stored.matches.slice(0, 1)
  fixture.restoreTest(stored)
  open(fixture)
  expect(
    await screen.findByRole('heading', { level: 2, name: '일부 AI가 사진을 관찰하지 못했어요' }),
  ).toBeVisible()
  expect(screen.getByRole('button', { name: '실패한 후보의 비용 확인' })).toBeVisible()
  expect(screen.getByRole('button', { name: '테스트 끝내기' })).toBeVisible()
  expect(screen.queryByRole('button', { name: /승자로 선택/ })).not.toBeInTheDocument()
  expect(fixture.admissions).toHaveLength(0)
})
