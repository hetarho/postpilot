import { useWritingTestTranslation } from '@/features/writing-test'
import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { BlockList } from '@/entities/post'
import {
  writingTestPayloadAvailable,
  type WritingTest,
  type WritingTestCandidate,
  type WritingTestMatch,
} from '@/entities/writing-test'
import { currentWritingTestMatch } from '@/features/writing-test'
import { toMarkdown } from '@/features/export-markdown'
import { toNaver } from '@/features/export-naver'
import { copyText } from '@/shared/lib'
import { ActionBar, Button, Notice, SegmentedControl, Textarea, Typography } from '@/shared/ui'

interface WritingTestPairProps {
  test: WritingTest
  match: WritingTestMatch
  pending?: boolean
  onWinner: (candidateId: string) => void
  visibleCandidateId?: string
  onShowCandidate: (candidateId: string) => void
  reading?: Record<string, number>
  onReading: (candidateId: string, position: number) => void
}

/** Exactly the server's current pair. Reading or copying never starts generation or decides a match. */
export function WritingTestPair({
  test,
  match,
  pending = false,
  onWinner,
  visibleCandidateId,
  onShowCandidate,
  reading,
  onReading,
}: WritingTestPairProps) {
  const { t } = useWritingTestTranslation()
  const panelId = useId()
  const pairTop = useRef<HTMLDivElement>(null)
  const previousCandidate = useRef('')
  const phoneReading = useRef<Record<string, number>>({})
  const readingCallback = useRef(onReading)
  useLayoutEffect(() => {
    readingCallback.current = onReading
  }, [onReading])
  const left = test.candidates.find((candidate) => candidate.id === match.leftCandidateId)
  const right = test.candidates.find((candidate) => candidate.id === match.rightCandidateId)
  const activeId = right && visibleCandidateId === right.id ? right.id : (left?.id ?? '')
  const payloadAvailable = writingTestPayloadAvailable(test)
  const allReady =
    test.candidates.length === test.count &&
    test.candidates.every((candidate) => candidate.status === 'succeeded' && !!candidate.output)
  const serverMatch = test.matches.find((candidate) => candidate.id === match.id)
  const current =
    serverMatch &&
    currentWritingTestMatch(test)?.id === match.id &&
    !serverMatch.winnerCandidateId &&
    !match.winnerCandidateId &&
    serverMatch.leftCandidateId === match.leftCandidateId &&
    serverMatch.rightCandidateId === match.rightCandidateId &&
    serverMatch.round === match.round &&
    serverMatch.index === match.index
  const canCompare =
    test.status === 'review' &&
    payloadAvailable &&
    allReady &&
    current &&
    left &&
    right &&
    left.id !== right.id

  useLayoutEffect(() => {
    if (!activeId) return
    if (window.matchMedia('(max-width: 767px)').matches && previousCandidate.current !== activeId) {
      const saved = phoneReading.current[activeId] ?? reading?.[activeId]
      if (typeof saved === 'number' && Number.isFinite(saved) && saved >= 0)
        window.scrollTo(0, saved)
      else if (previousCandidate.current && pairTop.current) {
        window.scrollTo(0, window.scrollY + pairTop.current.getBoundingClientRect().top)
      }
      phoneReading.current[activeId] = Math.max(0, window.scrollY)
    }
    previousCandidate.current = activeId
  }, [activeId, reading])

  useEffect(() => {
    if (!canCompare || !activeId) return
    const phone = window.matchMedia('(max-width: 767px)')
    let frame: number | undefined
    let wasPhone = phone.matches
    const record = () => {
      const position = Math.max(0, window.scrollY)
      // Chromium can emit a reflow scroll before the breakpoint change notification.
      if (phone.matches && wasPhone) phoneReading.current[activeId] = position
      readingCallback.current(activeId, position)
    }
    const restorePhoneReading = () => {
      wasPhone = phone.matches
      if (!phone.matches) return
      const saved = phoneReading.current[activeId]
      if (typeof saved !== 'number' || !Number.isFinite(saved) || saved < 0) return
      if (frame !== undefined) window.cancelAnimationFrame(frame)
      frame = window.requestAnimationFrame(() => {
        frame = undefined
        if (phone.matches) window.scrollTo(0, saved)
      })
    }
    window.addEventListener('scroll', record, { passive: true })
    phone.addEventListener('change', restorePhoneReading)
    return () => {
      window.removeEventListener('scroll', record)
      phone.removeEventListener('change', restorePhoneReading)
      if (frame !== undefined) window.cancelAnimationFrame(frame)
    }
  }, [activeId, canCompare])

  if (!payloadAvailable) {
    const expired = test.candidates.some(
      (candidate) => candidate.status === 'succeeded' || !!candidate.output,
    )
    return (
      <Notice tone={expired ? 'warning' : 'info'} role="status">
        {t(expired ? 'expired' : 'progress', { ready: 0, total: test.count })}
      </Notice>
    )
  }
  if (!canCompare || !left || !right)
    return (
      <Notice tone="info" role="status">
        {t(allReady ? 'waitingMatch' : 'progress', {
          ready: test.candidates.filter(
            (candidate) => candidate.status === 'succeeded' && !!candidate.output,
          ).length,
          total: test.count,
        })}
      </Notice>
    )
  const pair = [left, right]
  const labelOf = (candidate: WritingTestCandidate) =>
    String.fromCharCode(65 + test.candidates.findIndex((entry) => entry.id === candidate.id))
  return (
    <div ref={pairTop}>
      <Typography variant="body" as="p" className="text-content-secondary">
        {t('compareHelp')}
      </Typography>
      <Typography variant="label" as="p" className="mt-2">
        {t('frozenLanguage', { language: t(test.targetLanguage) })}
      </Typography>
      <div
        id={panelId}
        role="tabpanel"
        aria-label={t('compareTitle')}
        className="mt-6 grid items-start gap-6 md:grid-cols-2"
      >
        {pair.map((candidate) => (
          <section
            key={candidate.id}
            lang={test.targetLanguage}
            className={`${candidate.id === activeId ? 'block' : 'hidden'} min-w-0 md:block`}
            aria-label={t('candidate', { label: labelOf(candidate) })}
          >
            <Typography variant="title" as="h2">
              {t('candidate', { label: labelOf(candidate) })}
            </Typography>
            {test.revealed && candidate.identity && (
              <Typography variant="label" as="p" className="mt-2 break-words">
                {candidate.identity.label}
              </Typography>
            )}
            <WritingTestPost
              key={`${test.id}:${candidate.id}`}
              candidate={candidate}
              test={test}
              label={labelOf(candidate)}
            />
          </section>
        ))}
      </div>
      <ActionBar ariaLabel={t('compareTitle')}>
        <SegmentedControl
          className="mb-3 md:hidden"
          value={activeId}
          options={pair.map((candidate) => ({
            value: candidate.id,
            label: t('candidate', { label: labelOf(candidate) }),
          }))}
          controls={panelId}
          ariaLabel={t('readCandidate')}
          onChange={(candidateId) => {
            onReading(activeId, Math.max(0, window.scrollY))
            onShowCandidate(candidateId)
          }}
        />
        <div className="grid gap-3 sm:grid-cols-2">
          {pair.map((candidate) => (
            <Button
              key={candidate.id}
              variant="secondary"
              disabled={pending}
              onClick={() => {
                if (!pending && canCompare && writingTestPayloadAvailable(test))
                  onWinner(candidate.id)
              }}
            >
              {t('pickWinner', { label: t('candidate', { label: labelOf(candidate) }) })}
            </Button>
          ))}
        </div>
      </ActionBar>
    </div>
  )
}

export function WritingTestPost({
  candidate,
  test,
  label,
}: {
  candidate: WritingTestCandidate
  test: WritingTest
  label: string
}) {
  const { t } = useWritingTestTranslation()
  const [feedback, setFeedback] = useState<{
    value: string
    format: 'naver' | 'markdown'
    copied: boolean
  }>()
  const selectFallback = useCallback((element: HTMLTextAreaElement | null) => {
    element?.focus()
    element?.select()
  }, [])
  const content = candidate.output
  if (!content) return null
  const outputs = {
    naver: toNaver(content, [], test.targetLanguage),
    markdown: toMarkdown(content, [], test.createdAt, test.targetLanguage),
  }
  const currentFeedback =
    feedback && feedback.value === outputs[feedback.format] ? feedback : undefined
  return (
    <>
      <div className="mt-3 flex flex-wrap gap-2">
        {(['naver', 'markdown'] as const).map((format) => (
          <Button
            key={format}
            variant="ghost"
            onClick={() => {
              if (!writingTestPayloadAvailable(test)) return
              const value = outputs[format]
              void copyText(value).then(({ copied }) => setFeedback({ value, format, copied }))
            }}
          >
            {t(format === 'naver' ? 'copyNaver' : 'copyMarkdown')}
          </Button>
        ))}
      </div>
      {currentFeedback && (
        <Typography variant="label" as="p" role="status" className="mt-2">
          {t(currentFeedback.copied ? 'copied' : 'manualCopy')}
        </Typography>
      )}
      {currentFeedback && !currentFeedback.copied && (
        <Textarea
          aria-label={`${t('copy')} · ${label}`}
          readOnly
          autoGrow
          value={currentFeedback.value}
          rows={6}
          className="mt-2"
          ref={selectFallback}
        />
      )}
      <BlockList
        content={content}
        images={[]}
        videos={[]}
        label={t('candidate', { label })}
        className="mt-4 pb-4"
        renderMissingImage={(block) => (
          <div className="bg-surface-recessed rounded-lg p-4">
            <Typography variant="label" as="p" className="break-words">
              {t('attachment', { name: block.file })}
            </Typography>
            {block.caption && (
              <Typography variant="body" as="p" className="mt-2 break-words">
                {block.caption}
              </Typography>
            )}
          </div>
        )}
      />
    </>
  )
}
