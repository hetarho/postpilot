import { useId, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  useVoiceCreation,
  VoiceDefinition,
  type CreationAddress,
} from '@/features/create-spoken-voice'
import { SpokenSamplePlayer, type SpokenDraftInput } from '@/entities/spoken-voice'
import {
  ActionBar,
  Button,
  Dialog,
  RadioGroup,
  SegmentedControl,
  Typography,
  buttonStyles,
} from '@/shared/ui'
type Panel = 'details' | 'audition' | 'saved'
export function SpokenVoiceCreation({
  ownerId,
  address,
  onAddress,
  initialInput,
}: {
  ownerId: string
  address: CreationAddress
  onAddress: (next: CreationAddress) => void
  initialInput?: SpokenDraftInput
}) {
  const { t } = useTranslation('spokenCreation'),
    { t: action } = useTranslation('createSpokenVoice'),
    c = useVoiceCreation(ownerId, address, onAddress, initialInput),
    id = useId()
  const [view, setView] = useState<Panel | null>(null),
    [cancelOpen, setCancelOpen] = useState(false)
  const persisted: Panel = c.draft?.confirmedVoiceId
    ? 'saved'
    : c.draft?.candidates.length
      ? 'audition'
      : 'details'
  const panel = view ?? persisted
  const status = c.failure
    ? action('failed')
    : c.job.isError
      ? action('pollFailed')
      : c.query.isError
        ? t('draftFailed')
        : c.profiles.isError
          ? t('profileFailed')
          : c.running
            ? action('working')
            : c.job.operation?.state === 'received'
              ? action('received')
              : c.job.operation?.state === 'unresolved'
                ? action('unresolved')
                : c.job.operation?.state === 'failed'
                  ? action('failed')
                  : c.job.operation?.state === 'cancelled'
                    ? action('cancelled')
                    : c.note
                      ? action(c.note as 'saved' | 'complete' | 'playFailed')
                      : c.draft?.confirmedVoiceId
                        ? action('complete')
                        : c.dirty
                          ? action('saveChanges')
                          : c.input.profileId && !c.selected?.available
                            ? action('unavailable')
                            : ''
  return (
    <div className="min-w-0">
      <Typography variant="display">{c.draft?.name || t('title')}</Typography>
      <div className="bg-surface-base top-top-chrome sticky z-10 mt-4 pb-3">
        <SegmentedControl
          variant="steps"
          value={panel}
          ariaLabel={t('steps')}
          controls={id}
          options={(['details', 'audition', 'saved'] as const).map((value) => ({
            value,
            label: t(value),
          }))}
          onChange={setView}
        />
        <Typography
          variant="meta"
          as="p"
          role="status"
          aria-live="polite"
          className="mt-2 min-h-5 break-words"
        >
          {status || '\u00a0'}
        </Typography>
        {c.job.isError && (
          <Button variant="ghost" onClick={() => void c.job.refetch()}>
            {action('retryStatus')}
          </Button>
        )}
        {(c.query.isError || c.profiles.isError) && (
          <Button
            variant="ghost"
            onClick={() => {
              void c.query.refetch()
              void c.profiles.refetch()
            }}
          >
            {t('retry')}
          </Button>
        )}
      </div>
      <div id={id} role="tabpanel" aria-label={t(panel)} className="mt-6 min-w-0">
        {panel === 'details' && (
          <>
            <VoiceDefinition
              input={c.input}
              profiles={c.profiles.profiles}
              onChange={c.edit}
              disabled={
                c.busy || !!c.draft?.confirmedVoiceId || (!!address.draft && c.query.isPending)
              }
            />
            <ActionBar className="static">
              <div className="flex flex-wrap items-center gap-3">
                <Button
                  variant="ghost"
                  disabled={!c.valid || c.busy || !c.dirty || !!c.draft?.confirmedVoiceId}
                  onClick={() => void c.save()}
                >
                  {action('save')}
                </Button>
                <Button
                  variant="cta"
                  disabled={!c.valid || c.busy || !!c.draft?.confirmedVoiceId}
                  onClick={() => void c.estimate()}
                >
                  {action('estimate')}
                </Button>
              </div>
            </ActionBar>
          </>
        )}
        {panel === 'audition' &&
          (c.draft?.candidates.length ? (
            <>
              <RadioGroup
                label={t('audition')}
                value={c.draft.selectedCandidateId}
                disabled={c.busy || !!c.draft.confirmedVoiceId}
                onChange={(value) => void c.select(value)}
                options={c.draft.candidates.map((candidate, i) => ({
                  value: candidate.id,
                  label: action('choose', { number: i + 1 }),
                  detail: (
                    <SpokenSamplePlayer
                      ownerId={ownerId}
                      assetId={candidate.assetId}
                      name={action('candidate', { number: i + 1 })}
                      durationMs={candidate.durationMs}
                      disabled={c.busy}
                      onPlayed={async (playback) => {
                        await c.acknowledge(candidate.id, playback)
                      }}
                      onFailure={c.playbackFailed}
                    />
                  ),
                }))}
              />
              <ActionBar className="static">
                <Button
                  variant="cta"
                  disabled={!c.confirmable || !!c.draft.confirmedVoiceId}
                  onClick={() => void c.estimate(true)}
                >
                  {action('confirmEstimate')}
                </Button>
                {!c.confirmable && !c.busy && !c.draft.confirmedVoiceId && (
                  <Typography variant="meta" as="p" className="mt-2">
                    {c.dirty ? action('saveChanges') : action('listenFirst')}
                  </Typography>
                )}
              </ActionBar>
            </>
          ) : (
            <>
              <Typography variant="body">{t('waitingCandidates')}</Typography>
              <Button variant="ghost" onClick={() => setView('details')}>
                {t('backDetails')}
              </Button>
            </>
          ))}
        {panel === 'saved' &&
          (c.draft?.confirmedVoiceId ? (
            <Link to="/spoken-voices" className={buttonStyles({ variant: 'cta' })}>
              {t('library')}
            </Link>
          ) : (
            <>
              <Typography variant="body">{t('waitingSaved')}</Typography>
              <Button variant="ghost" onClick={() => setView('audition')}>
                {t('backAudition')}
              </Button>
            </>
          ))}
        {c.running && (
          <Button variant="ghost" onClick={() => setCancelOpen(true)} className="mt-4">
            {action('cancel')}
          </Button>
        )}
        {c.job.operation?.state === 'received' && (
          <Button variant="secondary" onClick={() => void c.recover()} disabled={c.busy}>
            {action('recover')}
          </Button>
        )}
      </div>
      <Dialog
        open={!!c.approval}
        title={action('approvalTitle')}
        confirmLabel={action('start')}
        pending={c.busy}
        onClose={c.closeApproval}
        onConfirm={() => {
          setView(null)
          void c.start()
        }}
      >
        <Typography variant="fieldTitle">{c.draft?.name}</Typography>
        <Typography variant="meta" as="p" className="mt-2 break-words">
          {c.draft?.profile.designLabel} → {c.draft?.profile.speechLabel}
        </Typography>
        {!!c.failure && (
          <Typography variant="body" className="mt-3">
            {action('failed')} {action('retryStart')}
          </Typography>
        )}
        <Typography variant="body" className="mt-3">
          {c.approval?.quote.maximumCredits === 0 && c.approval.input.kind === 'voice_confirm'
            ? action('confirmFree')
            : action('approval', { credits: c.approval?.quote.maximumCredits ?? 0 })}
        </Typography>
        <Typography variant="body" className="mt-3">
          {action('approvalExplain')}
        </Typography>
      </Dialog>
      <Dialog
        open={cancelOpen}
        title={action('cancelTitle')}
        confirmLabel={action('cancel')}
        onClose={() => setCancelOpen(false)}
        onConfirm={() => {
          setCancelOpen(false)
          void c.cancel()
        }}
      >
        <Typography variant="body">{action('cancelExplain')}</Typography>
      </Dialog>
    </div>
  )
}
