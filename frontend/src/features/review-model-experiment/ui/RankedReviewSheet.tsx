import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  BADGE_NOTE_MAX_LENGTH,
  candidateSides,
  NEGATIVE_BADGES,
  POSITIVE_BADGES,
  badgeAppliesTo,
  type ModelExperiment,
  type VerdictBadgeName,
} from '@/entities/model-experiment'
import type { AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  ChipToggle,
  FieldCount,
  FieldLabel,
  Listbox,
  Notice,
  Sheet,
  Textarea,
  Typography,
} from '@/shared/ui'
import { completeRanks } from '../model/ranking-review'

export interface RankedReviewAnswer {
  candidateId: string
  rank: number
  badges: VerdictBadgeName[]
  otherNote: string
}

export function RankedReviewSheet({
  experiment,
  open,
  pending,
  failure,
  onConfirm,
  onClose,
}: {
  experiment: ModelExperiment
  open: boolean
  pending: boolean
  failure?: AppFailure
  onConfirm: (ranks: RankedReviewAnswer[]) => Promise<unknown>
  onClose: () => void
}) {
  const { t } = useTranslation(['models', 'common'])
  const sides = candidateSides(experiment.candidates).filter(
    ({ candidate }) => candidate.status === 'succeeded',
  )
  const [ranks, setRanks] = useState<Record<string, number>>({})
  const [picked, setPicked] = useState<Record<string, VerdictBadgeName[]>>({})
  const [notes, setNotes] = useState<Record<string, string>>({})
  const complete = completeRanks(
    sides.map(({ candidate }) => candidate),
    ranks,
  )
  const overLongNote = sides.some(
    ({ candidate }) =>
      (picked[candidate.id] ?? []).includes('other') &&
      (notes[candidate.id] ?? '').length > BADGE_NOTE_MAX_LENGTH,
  )
  const toggle = (id: string, badge: VerdictBadgeName) =>
    setPicked((previous) => {
      const list = previous[id] ?? []
      return {
        ...previous,
        [id]: list.includes(badge) ? list.filter((item) => item !== badge) : [...list, badge],
      }
    })
  const submit = () => {
    if (!complete || overLongNote || pending) return
    void onConfirm(
      complete.map(({ candidateId, rank }) => ({
        candidateId,
        rank,
        badges: picked[candidateId] ?? [],
        otherNote: (picked[candidateId] ?? []).includes('other') ? (notes[candidateId] ?? '') : '',
      })),
    )
      .then(onClose)
      .catch(() => {
        // The same local ranks and badges remain open beside the structured refusal.
      })
  }
  return (
    <Sheet
      open={open}
      label={t('ranking.title')}
      onClose={() => {
        if (!pending) onClose()
      }}
      header={
        <div>
          <Typography variant="title">{t('ranking.title')}</Typography>
          <Typography variant="body" as="p" className="text-content-secondary mt-1">
            {t('ranking.instructions')}
          </Typography>
        </div>
      }
      footer={
        <div className="grid gap-3">
          {!complete && (
            <Typography variant="label" as="p" role="status">
              {t('ranking.incomplete')}
            </Typography>
          )}
          {failure && (
            <Notice tone="danger" role="alert">
              <AppFailureMessage failure={failure} />
            </Notice>
          )}
          <div className="flex justify-end gap-3">
            <Button variant="ghost" disabled={pending} onClick={onClose}>
              {t('action.cancel', { ns: 'common' })}
            </Button>
            <Button
              variant="cta"
              pending={pending}
              disabled={!complete || overLongNote}
              onClick={submit}
            >
              {t('ranking.save')}
            </Button>
          </div>
        </div>
      }
    >
      <div className="grid gap-6">
        {sides.map(({ candidate, label }) => {
          const badges = picked[candidate.id] ?? []
          const ranked = Boolean(ranks[candidate.id])
          return (
            <section key={candidate.id} className="grid gap-3">
              <div>
                <FieldLabel htmlFor={`rank-${candidate.id}`}>
                  {t('ranking.candidate', { label })}
                </FieldLabel>
                <Listbox
                  id={`rank-${candidate.id}`}
                  className="mt-1"
                  value={String(ranks[candidate.id] ?? '')}
                  options={[
                    { value: '', label: t('ranking.select') },
                    ...sides.map((_, index) => ({
                      value: String(index + 1),
                      label: t('ranking.place', { rank: index + 1 }),
                    })),
                  ]}
                  onChange={(value) =>
                    setRanks((previous) => ({ ...previous, [candidate.id]: Number(value) }))
                  }
                />
              </div>
              <BadgeChoices
                heading={t('verdict.positive')}
                badges={POSITIVE_BADGES.filter((badge) => badgeAppliesTo(badge, experiment.stage))}
                picked={badges}
                disabled={!ranked}
                onToggle={(badge) => toggle(candidate.id, badge)}
              />
              <BadgeChoices
                heading={t('verdict.negative')}
                badges={NEGATIVE_BADGES.filter((badge) => badgeAppliesTo(badge, experiment.stage))}
                picked={badges}
                disabled={!ranked}
                onToggle={(badge) => toggle(candidate.id, badge)}
              />
              <ChipToggle
                pressed={badges.includes('other')}
                disabled={!ranked}
                onClick={() => toggle(candidate.id, 'other')}
                className="justify-self-start"
              >
                {t('badge.other')}
              </ChipToggle>
              {badges.includes('other') && (
                <div>
                  <FieldLabel htmlFor={`rank-note-${candidate.id}`}>
                    {t('verdict.noteLabel')}
                  </FieldLabel>
                  <Textarea
                    id={`rank-note-${candidate.id}`}
                    rows={2}
                    autoGrow
                    value={notes[candidate.id] ?? ''}
                    onChange={(event) =>
                      setNotes((previous) => ({ ...previous, [candidate.id]: event.target.value }))
                    }
                  />
                  <FieldCount left={BADGE_NOTE_MAX_LENGTH - (notes[candidate.id] ?? '').length} />
                </div>
              )}
            </section>
          )
        })}
      </div>
    </Sheet>
  )
}

function BadgeChoices({
  heading,
  badges,
  picked,
  disabled,
  onToggle,
}: {
  heading: string
  badges: readonly VerdictBadgeName[]
  picked: readonly VerdictBadgeName[]
  disabled: boolean
  onToggle: (badge: VerdictBadgeName) => void
}) {
  const { t } = useTranslation('models')
  return (
    <div>
      <Typography variant="meta" as="p" className="mb-2">
        {heading}
      </Typography>
      <div className="flex flex-wrap gap-2">
        {badges.map((badge) => (
          <ChipToggle
            key={badge}
            pressed={picked.includes(badge)}
            disabled={disabled}
            onClick={() => onToggle(badge)}
          >
            {t(`badge.${badge}`)}
          </ChipToggle>
        ))}
      </div>
    </div>
  )
}
