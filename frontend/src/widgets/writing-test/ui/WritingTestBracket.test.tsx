import { render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { beforeEach, expect, it } from 'vitest'
import type { WritingTest } from '@/entities/writing-test'
import { writingTestI18n } from '@/features/writing-test'
import { WritingTestBracket } from './WritingTestBracket'

beforeEach(() => i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true))
const fixture = () =>
  ({
    count: 4,
    candidates: ['a', 'b', 'c', 'd'].map((id) => ({ id })),
    matches: [
      {
        id: 'final',
        round: 2,
        index: 0,
        leftCandidateId: 'a',
        rightCandidateId: '',
        winnerCandidateId: '',
      },
      {
        id: 'semi-2',
        round: 1,
        index: 1,
        leftCandidateId: 'c',
        rightCandidateId: 'd',
        winnerCandidateId: '',
      },
      {
        id: 'semi-1',
        round: 1,
        index: 0,
        leftCandidateId: 'a',
        rightCandidateId: 'b',
        winnerCandidateId: 'a',
      },
    ],
  }) as WritingTest

it('renders stable stored pairs and progress without inventing a next contestant or champion', () => {
  render(<WritingTestBracket test={fixture()} />)
  const rows = within(screen.getByRole('region', { name: '대결 진행' })).getAllByRole('listitem')
  expect(rows).toHaveLength(3)
  expect(rows[0]).toHaveTextContent('1라운드 · 1번 대결')
  expect(rows[0]).toHaveTextContent('후보 A · 후보 B')
  expect(rows[0]).toHaveTextContent('승자 후보 A')
  expect(rows[1]).toHaveTextContent('후보 C · 후보 D')
  expect(rows[2]).toHaveTextContent('앞 대결의 승자를 기다리고 있어요')
  expect(screen.getByRole('status')).toHaveTextContent('1 / 3번 선택 완료')
  expect(screen.queryByText(/우승|Elo|순위/)).not.toBeInTheDocument()
})

it('keeps the same match row nodes when a server decision fills an awaiting final slot', () => {
  const test = fixture()
  const rendered = render(<WritingTestBracket test={test} />)
  const first = screen.getAllByRole('listitem')[0]
  const next = {
    ...test,
    matches: test.matches.map((match) =>
      match.id === 'semi-2'
        ? { ...match, winnerCandidateId: 'd' }
        : match.id === 'final'
          ? { ...match, rightCandidateId: 'd' }
          : match,
    ),
  }
  rendered.rerender(<WritingTestBracket test={next} />)
  expect(screen.getAllByRole('listitem')[0]).toBe(first)
  expect(screen.getByRole('status')).toHaveTextContent('2 / 3번 선택 완료')
  expect(screen.getAllByRole('listitem')[2]).toHaveTextContent('후보 A · 후보 D')
  expect(screen.getAllByRole('listitem')[2]).not.toHaveTextContent('승자')
})

it('does not synthesize a bracket or champion when server match metadata is absent', () => {
  render(<WritingTestBracket test={{ ...fixture(), matches: [], winnerCandidateId: 'a' }} />)
  expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
  expect(screen.queryByText(/승자 후보|우승/)).not.toBeInTheDocument()
})
