import { useId, useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import {
  writingTestCandidateReadable,
  writingTestPayloadAvailable,
  type WritingTest,
  type WritingTestCandidate,
  type WritingTestMatch,
  type WritingTestPhoto,
} from '@/entities/writing-test'
import { currentWritingTestMatch, useWritingTestTranslation } from '@/features/writing-test'
import { ActionBar, Button, Carousel, Menu, Notice, Typography, cardStyles } from '@/shared/ui'
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

/** Exactly the server's current pair, one photo and one candidate's card at a time. Reading never
 *  starts work or decides a match; the contestants stay blind until the champion is confirmed
 *  (MODEL-32). */
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
        <WritingTestObservationViewer key={match.id} test={test} columns={columns} />
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

/** One frozen photo at a time, held still beside one column's observation card, named like
 *  후보 A · 사진 3 (MODEL-97). The photo switcher and the candidate switcher each step previous/next
 *  and open a menu of every photo or candidate; a sideways swipe of the card moves to the other
 *  candidate. Every photo and every card stays mounted, so a switch shows at once. One column —
 *  the champion, a done contestant — reads the same viewer without the candidate switcher. */
export function WritingTestObservationViewer({
  test,
  columns,
}: {
  test: WritingTest
  columns: ObservationColumn[]
}) {
  const { t } = useWritingTestTranslation()
  const photos = test.photos ?? []
  const [photoIndex, setPhotoIndex] = useState(0)
  const [columnIndex, setColumnIndex] = useState(0)
  const current = Math.min(photoIndex, Math.max(photos.length - 1, 0))
  const column = Math.min(columnIndex, Math.max(columns.length - 1, 0))
  const photo = photos[current]
  if (!photo || !columns.length) return null
  return (
    <div className="grid min-w-0 gap-4 md:grid-cols-2 md:items-start">
      <div className="flex min-w-0 flex-col gap-2">
        <ObservationSwitcher
          label={t('observation.photoMenu')}
          previousLabel={t('observation.previousPhoto')}
          nextLabel={t('observation.nextPhoto')}
          triggerLabel={t('observation.photoPosition', {
            current: current + 1,
            total: photos.length,
          })}
          index={current}
          options={photos.map((each, index) =>
            t('observation.photoName', { number: index + 1, name: each.filename }),
          )}
          onChange={setPhotoIndex}
        />
        <div className="bg-surface-recessed flex h-64 min-w-0 items-center justify-center overflow-hidden rounded-lg sm:h-80">
          {photos.map((each, index) => (
            <ObservationPhoto key={each.id} photo={each} hidden={index !== current} />
          ))}
        </div>
      </div>
      <div className="flex min-w-0 flex-col gap-2">
        {columns.length > 1 && (
          <ObservationSwitcher
            label={t('observation.candidateMenu')}
            previousLabel={t('observation.previousCandidate')}
            nextLabel={t('observation.nextCandidate')}
            triggerLabel={t('candidate', { label: columns[column]!.label })}
            index={column}
            options={columns.map(({ label }) => t('candidate', { label }))}
            onChange={setColumnIndex}
          />
        )}
        <Carousel
          index={column}
          onIndexChange={setColumnIndex}
          label={t('observation.cards')}
          className="-mx-1 gap-3 p-1"
        >
          {columns.map(({ candidate, label }) => (
            <ObservationCard
              key={candidate.id}
              test={test}
              candidate={candidate}
              label={label}
              photo={photo}
              number={current + 1}
            />
          ))}
        </Carousel>
      </div>
    </div>
  )
}

/** ‹ the current photo or candidate as a menu › — the previous and next steps stop at the ends. */
function ObservationSwitcher({
  label,
  previousLabel,
  nextLabel,
  triggerLabel,
  index,
  options,
  onChange,
}: {
  label: string
  previousLabel: string
  nextLabel: string
  triggerLabel: string
  index: number
  options: readonly string[]
  onChange: (index: number) => void
}) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2">
      <Button
        variant="ghost"
        size="icon"
        aria-label={previousLabel}
        title={previousLabel}
        disabled={index === 0}
        onClick={() => onChange(index - 1)}
      >
        <ChevronLeft aria-hidden="true" className="size-5" />
      </Button>
      <Menu
        label={label}
        triggerLabel={triggerLabel}
        triggerIcon={null}
        value={String(index)}
        options={options.map((option, position) => ({ value: String(position), label: option }))}
        onChange={(value) => onChange(Number(value))}
      />
      <Button
        variant="ghost"
        size="icon"
        aria-label={nextLabel}
        title={nextLabel}
        disabled={index >= options.length - 1}
        onClick={() => onChange(index + 1)}
      >
        <ChevronRight aria-hidden="true" className="size-5" />
      </Button>
    </div>
  )
}

function ObservationPhoto({ photo, hidden }: { photo: WritingTestPhoto; hidden: boolean }) {
  const { t } = useWritingTestTranslation()
  // A signed view link can lapse while the page stays open; the frame then says so.
  const [failedUrl, setFailedUrl] = useState('')
  const url = photo.viewUrl ?? ''
  if (!url || failedUrl === url)
    return (
      <Typography variant="label" as="p" role="status" hidden={hidden} className="p-4">
        {t('observation.photoUnavailable')}
      </Typography>
    )
  return (
    <img
      src={url}
      alt={t('observation.photoAlt', { name: photo.filename })}
      width={photo.width || undefined}
      height={photo.height || undefined}
      hidden={hidden}
      className="h-auto max-h-full w-auto max-w-full object-contain"
      onError={() => setFailedUrl(url)}
    />
  )
}

function ObservationCard({
  test,
  candidate,
  label,
  photo,
  number,
}: {
  test: WritingTest
  candidate: WritingTestCandidate
  label: string
  photo: WritingTestPhoto
  number: number
}) {
  const { t } = useWritingTestTranslation()
  const titleId = useId()
  const observation = candidate.observations?.find((entry) => entry.file === photo.filename)
  const none = t('observation.absent')
  return (
    <section
      aria-labelledby={titleId}
      className={cardStyles({ padding: 'sm', className: 'h-full min-w-0 shadow-sm' })}
    >
      <Typography id={titleId} variant="fieldTitle" as="p" className="break-words">
        {t('observation.cardTitle', { candidate: t('candidate', { label }), number })}
        {test.revealed && candidate.identity ? ` · ${candidate.identity.label}` : ''}
      </Typography>
      {observation ? (
        <dl className="mt-3 grid gap-2">
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
        <Typography variant="body" as="p" className="text-content-secondary mt-3">
          {t('observation.missing')}
        </Typography>
      )}
    </section>
  )
}
