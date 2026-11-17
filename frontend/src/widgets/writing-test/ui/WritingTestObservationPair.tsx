import { useState } from 'react'
import {
  writingTestCandidateReadable,
  writingTestPayloadAvailable,
  type WritingTest,
  type WritingTestCandidate,
  type WritingTestMatch,
  type WritingTestPhoto,
} from '@/entities/writing-test'
import { currentWritingTestMatch, useWritingTestTranslation } from '@/features/writing-test'
import { ActionBar, Button, Notice, Typography } from '@/shared/ui'
import { writingTestCandidateLabel } from '../lib/candidate-label'

const readyCandidates = (test: WritingTest) =>
  test.candidates.filter(
    (candidate) => candidate.status === 'succeeded' && writingTestCandidateReadable(candidate),
  ).length

interface ObservationColumn {
  candidate: WritingTestCandidate
  label: string
}

interface WritingTestObservationPairProps {
  test: WritingTest
  match: WritingTestMatch
  pending?: boolean
  onWinner: (candidateId: string) => void
}

/** Exactly the server's current pair, photo by photo. Reading never starts work or decides a match;
 *  the contestants stay blind until the champion is confirmed (MODEL-32). */
export function WritingTestObservationPair({
  test,
  match,
  pending = false,
  onWinner,
}: WritingTestObservationPairProps) {
  const { t } = useWritingTestTranslation()
  const left = test.candidates.find((candidate) => candidate.id === match.leftCandidateId)
  const right = test.candidates.find((candidate) => candidate.id === match.rightCandidateId)
  const allReady = readyCandidates(test) === test.count && test.candidates.length === test.count
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
  const payloadAvailable = writingTestPayloadAvailable(test)
  const canCompare =
    test.status === 'review' &&
    payloadAvailable &&
    allReady &&
    current &&
    left &&
    right &&
    left.id !== right.id

  if (!payloadAvailable) {
    const expired = test.candidates.some(
      (candidate) => candidate.status === 'succeeded' || writingTestCandidateReadable(candidate),
    )
    return (
      <Notice tone={expired ? 'warning' : 'info'} role="status">
        {expired
          ? t('observation.expired')
          : t('observation.progress', { ready: 0, total: test.count })}
      </Notice>
    )
  }
  if (!canCompare || !left || !right)
    return (
      <Notice tone="info" role="status">
        {allReady
          ? t('waitingMatch')
          : t('observation.progress', { ready: readyCandidates(test), total: test.count })}
      </Notice>
    )
  const pair = [left, right]
  const columns = pair.map((candidate) => ({
    candidate,
    label: writingTestCandidateLabel(test, candidate.id),
  }))
  return (
    <div>
      <Typography variant="body" as="p" className="text-content-secondary">
        {t('observation.compareHelp')}
      </Typography>
      <div role="group" aria-label={t('observation.compareTitle')} className="mt-6">
        <WritingTestObservationRows test={test} columns={columns} />
      </div>
      <ActionBar ariaLabel={t('observation.compareTitle')}>
        <div className="grid gap-3 sm:grid-cols-2">
          {columns.map(({ candidate, label }) => (
            <Button
              key={candidate.id}
              variant="secondary"
              disabled={pending}
              onClick={() => {
                if (!pending && canCompare && writingTestPayloadAvailable(test))
                  onWinner(candidate.id)
              }}
            >
              {t('pickWinner', { label: t('candidate', { label }) })}
            </Button>
          ))}
        </div>
      </ActionBar>
    </div>
  )
}

/** One row per frozen photo, with each column's observation of it: side by side from `sm:` up,
 *  stacked under the photo on phones so the same photo is never compared out of view. */
export function WritingTestObservationRows({
  test,
  columns,
}: {
  test: WritingTest
  columns: ObservationColumn[]
}) {
  const { t } = useWritingTestTranslation()
  return (
    <ol className="divide-divider grid gap-0 divide-y">
      {(test.photos ?? []).map((photo, index) => (
        <li key={photo.id} className="grid min-w-0 gap-4 py-6 first:pt-0">
          <Typography variant="fieldTitle" as="h3" className="break-words">
            {t('observation.photoName', { number: index + 1, name: photo.filename })}
          </Typography>
          <ObservationPhoto photo={photo} />
          <div className={`grid items-start gap-4 ${columns.length > 1 ? 'sm:grid-cols-2' : ''}`}>
            {columns.map(({ candidate, label }) => (
              <ObservationCell
                key={candidate.id}
                test={test}
                candidate={candidate}
                label={label}
                photo={photo}
              />
            ))}
          </div>
        </li>
      ))}
    </ol>
  )
}

function ObservationPhoto({ photo }: { photo: WritingTestPhoto }) {
  const { t } = useWritingTestTranslation()
  // A signed view link can lapse while the page stays open; the frame then says so.
  const [failedUrl, setFailedUrl] = useState('')
  const url = photo.viewUrl ?? ''
  if (!url || failedUrl === url)
    return (
      <div className="bg-surface-recessed rounded-lg p-4">
        <Typography variant="label" as="p" role="status">
          {t('observation.photoUnavailable')}
        </Typography>
      </div>
    )
  return (
    <img
      src={url}
      alt={t('observation.photoAlt', { name: photo.filename })}
      width={photo.width || undefined}
      height={photo.height || undefined}
      className="bg-surface-recessed h-auto max-h-80 w-auto max-w-full rounded-lg object-contain"
      onError={() => setFailedUrl(url)}
    />
  )
}

function ObservationCell({
  test,
  candidate,
  label,
  photo,
}: {
  test: WritingTest
  candidate: WritingTestCandidate
  label: string
  photo: WritingTestPhoto
}) {
  const { t } = useWritingTestTranslation()
  const observation = candidate.observations?.find((entry) => entry.file === photo.filename)
  const none = t('observation.absent')
  return (
    <section aria-label={`${t('candidate', { label })} · ${photo.filename}`} className="min-w-0">
      <Typography variant="label" as="p" className="text-content-primary break-words">
        {t('candidate', { label })}
        {test.revealed && candidate.identity ? ` · ${candidate.identity.label}` : ''}
      </Typography>
      {observation ? (
        <dl className="mt-2 grid gap-2">
          {[
            { key: 'scene', value: observation.scene || none },
            { key: 'mood', value: observation.mood || none },
            { key: 'visibleText', value: observation.visibleText || none },
            { key: 'objects', value: observation.objects.join(', ') || none },
            { key: 'people', value: observation.peoplePresent ? t('observation.present') : none },
          ].map(({ key, value }) => (
            <div key={key}>
              <Typography variant="meta" as="dt" className="text-content-tertiary">
                {t(`observation.${key}`)}
              </Typography>
              <Typography variant="body" as="dd" className="break-words whitespace-pre-wrap">
                {value}
              </Typography>
            </div>
          ))}
        </dl>
      ) : (
        <Typography variant="body" as="p" className="text-content-secondary mt-2">
          {t('observation.missing')}
        </Typography>
      )}
    </section>
  )
}
