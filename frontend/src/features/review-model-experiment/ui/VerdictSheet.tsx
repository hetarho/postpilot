import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  BADGE_NOTE_MAX_LENGTH,
  candidateSides,
  NEGATIVE_BADGES,
  POSITIVE_BADGES,
  badgeAppliesTo,
  type CandidateBadges,
  type ModelExperiment,
  type VerdictBadgeName,
} from '@/entities/model-experiment'
import {
  Button,
  ChipToggle,
  FieldCount,
  FieldLabel,
  Sheet,
  Textarea,
  Typography,
} from '@/shared/ui'

/** The one confirmation a winner action passes through. Its confirm is the single committing
 *  action — the verdict, plus whatever the comparison's origin makes that verdict do — and
 *  its cancel changes nothing (MODEL-61).
 *
 *  Both candidates are offered the catalog, chosen and unchosen alike, because the reason a
 *  result lost is worth as much as the reason the other won. They stay blind here: the sheet
 *  names them by the same A/B labels the panels behind it use, and the models are revealed
 *  only once the verdict is recorded (MODEL-32).
 *
 *  Zero badges confirm as readily as ten. The confirm is never gated on a selection — a badge
 *  is evidence offered, not a toll on the verdict. */
export function VerdictSheet({
  experiment,
  chosenCandidateId,
  title,
  confirmLabel,
  open,
  pending,
  onConfirm,
  onClose,
}: {
  experiment: ModelExperiment
  chosenCandidateId: string
  title: string
  confirmLabel: string
  open: boolean
  pending: boolean
  onConfirm: (badges: CandidateBadges[]) => void
  onClose: () => void
}) {
  const { t } = useTranslation(['models', 'common'])
  const [picked, setPicked] = useState<Record<string, VerdictBadgeName[]>>({})
  const [notes, setNotes] = useState<Record<string, string>>({})
  const sides = candidateSides(experiment.candidates)

  const toggle = (candidateId: string, badge: VerdictBadgeName) =>
    setPicked((current) => {
      const chosen = current[candidateId] ?? []
      return {
        ...current,
        [candidateId]: chosen.includes(badge)
          ? chosen.filter((entry) => entry !== badge)
          : [...chosen, badge],
      }
    })

  const overLongNote = Object.entries(notes).some(
    ([candidateId, note]) =>
      (picked[candidateId] ?? []).includes('other') && note.length > BADGE_NOTE_MAX_LENGTH,
  )

  const confirm = () =>
    onConfirm(
      sides
        .map(({ candidate }) => ({
          candidateId: candidate.id,
          badges: picked[candidate.id] ?? [],
          otherNote: (picked[candidate.id] ?? []).includes('other')
            ? (notes[candidate.id] ?? '')
            : '',
        }))
        .filter((entry) => entry.badges.length > 0),
    )

  return (
    <Sheet
      open={open}
      label={title}
      onClose={onClose}
      header={
        <div>
          <Typography variant="title">{title}</Typography>
          <Typography variant="meta" as="p" className="mt-1">
            {t('verdict.badgesOptional', { ns: 'models' })}
          </Typography>
        </div>
      }
      footer={
        <div className="flex justify-end gap-3">
          <Button variant="ghost" onClick={onClose} disabled={pending}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
          <Button variant="cta" onClick={confirm} pending={pending} disabled={overLongNote}>
            {confirmLabel}
          </Button>
        </div>
      }
    >
      <div className="grid gap-6">
        {sides.map(({ candidate, label }) => (
          <section key={candidate.id} className="grid gap-3">
            <Typography variant="label" as="h3">
              {t(candidate.id === chosenCandidateId ? 'verdict.chosen' : 'verdict.unchosen', {
                ns: 'models',
                label,
              })}
            </Typography>
            <BadgeGroup
              heading={t('verdict.positive', { ns: 'models' })}
              badges={POSITIVE_BADGES.filter((badge) => badgeAppliesTo(badge, experiment.stage))}
              picked={picked[candidate.id] ?? []}
              onToggle={(badge) => toggle(candidate.id, badge)}
            />
            <BadgeGroup
              heading={t('verdict.negative', { ns: 'models' })}
              badges={NEGATIVE_BADGES.filter((badge) => badgeAppliesTo(badge, experiment.stage))}
              picked={picked[candidate.id] ?? []}
              onToggle={(badge) => toggle(candidate.id, badge)}
            />
            {/* `other` sits below both groups: it is not a judgement of the same kind, and it
                is the only one that opens a field. */}
            <div className="grid gap-2">
              <ChipToggle
                pressed={(picked[candidate.id] ?? []).includes('other')}
                onClick={() => toggle(candidate.id, 'other')}
                className="justify-self-start"
              >
                {t('badge.other', { ns: 'models' })}
              </ChipToggle>
              {(picked[candidate.id] ?? []).includes('other') && (
                <div>
                  <FieldLabel htmlFor={`note-${candidate.id}`}>
                    {t('verdict.noteLabel', { ns: 'models' })}
                  </FieldLabel>
                  <Textarea
                    id={`note-${candidate.id}`}
                    rows={2}
                    autoGrow
                    value={notes[candidate.id] ?? ''}
                    onChange={(event) =>
                      setNotes((current) => ({ ...current, [candidate.id]: event.target.value }))
                    }
                  />
                  <FieldCount left={BADGE_NOTE_MAX_LENGTH - (notes[candidate.id] ?? '').length} />
                </div>
              )}
            </div>
          </section>
        ))}
      </div>
    </Sheet>
  )
}

function BadgeGroup({
  heading,
  badges,
  picked,
  onToggle,
}: {
  heading: string
  badges: readonly VerdictBadgeName[]
  picked: VerdictBadgeName[]
  onToggle: (badge: VerdictBadgeName) => void
}) {
  const { t } = useTranslation('models')
  return (
    <div>
      <Typography variant="meta" as="p" className="mb-2">
        {heading}
      </Typography>
      {/* Wrapping rather than scrolling: every option has to be reachable without a gesture
          that competes with the sheet's own dismissal (THEME-25). */}
      <div className="flex flex-wrap gap-2">
        {badges.map((badge) => (
          <ChipToggle key={badge} pressed={picked.includes(badge)} onClick={() => onToggle(badge)}>
            {t(`badge.${badge}`)}
          </ChipToggle>
        ))}
      </div>
    </div>
  )
}
