import { useId, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  type ComparisonPair,
  type ModelRef,
  sameRef,
  useStageSelection,
} from '@/entities/model-catalog'
import { useStartVoiceReflection } from '@/entities/model-experiment'
import { useSession } from '@/entities/session'
import {
  activeVoices,
  sortVoices,
  useVoicePrompts,
  useVoiceProfile,
  useVoices,
} from '@/entities/voice'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Listbox,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** The 말투 반영 tab's start (MODEL-41, MODEL-67): a made voice — the 기본 first and chosen — one of
 *  its answered prompts, and the saved write pair above. A photo prompt is listed but not
 *  choosable unless both write models read images, and the reason says so. */
export function VoiceReflectionStart({
  pair,
  refs,
  pairPending,
  pairSaving,
}: {
  pair: ComparisonPair | undefined
  refs: ModelRef[] | undefined
  pairPending: boolean
  pairSaving: boolean
}) {
  const { t } = useTranslation('models')
  const hintId = useId()
  const navigate = useNavigate()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { voices, isPending: voicesPending } = useVoices(ownerId)
  const made = sortVoices(activeVoices(voices).filter((voice) => voice.made))
  const [voiceId, setVoiceId] = useState('')
  const voice = made.find((candidate) => candidate.id === voiceId) ?? made[0]
  const { profile, isPending: profilePending } = useVoiceProfile(ownerId, voice?.id ?? '')
  const { prompts } = useVoicePrompts()
  const answered = new Set(
    (profile?.samples ?? [])
      .filter((sample) => sample.kind === 'answer')
      .map((sample) => sample.promptKey),
  )
  const answeredPrompts = prompts.filter((prompt) => answered.has(prompt.key))
  const write = useStageSelection('write')
  const candidate = (ref: ModelRef | undefined) =>
    ref ? write.models.find((model) => sameRef(model.ref, ref) && !model.disabled) : undefined
  const writeA = candidate(
    pair?.candidateA && !pair.candidateA.missing ? pair.candidateA.ref : undefined,
  )
  const writeB = candidate(
    pair?.candidateB && !pair.candidateB.missing ? pair.candidateB.ref : undefined,
  )
  const candidates = refs?.map((ref) => candidate(ref))
  const allReadImages = Boolean(candidates?.every((item) => item?.vision))
  const choosable = answeredPrompts.filter((prompt) => !prompt.photo || allReadImages)
  const [promptKey, setPromptKey] = useState('')
  const prompt = choosable.find((item) => item.key === promptKey) ?? choosable[0]
  const start = useStartVoiceReflection()

  const reason = voicesPending
    ? t('page.voice.checking')
    : !voice
      ? t('page.voice.noVoice')
      : profilePending
        ? t('page.voice.checking')
        : answeredPrompts.length === 0
          ? t('page.voice.noAnswer')
          : !prompt
            ? t('page.voice.photoOnly')
            : pairSaving
              ? t('page.pairSaving')
              : pairPending || write.isPending
                ? t('page.modelChecking')
                : !writeA || !writeB || !refs || candidates?.some((item) => !item)
                  ? t('page.requirement.pair')
                  : ''
  const canStart = !reason && !start.isPending

  const begin = async () => {
    if (!canStart || !voice || !prompt || !writeA || !writeB || !refs) return
    try {
      const response = await start.start(
        voice.id,
        prompt.key,
        writeA.ref,
        writeB.ref,
        refs.slice(2),
      )
      void navigate({
        to: '/tests/records/$id',
        params: { id: response.experimentId },
        search: { entry: '/tests' },
      })
    } catch {
      // The refusal renders beside the action.
    }
  }

  return (
    <>
      <div className="mt-6">
        <FieldLabel id="reflection-voice-label" htmlFor="reflection-voice">
          {t('page.voice.voice')}
        </FieldLabel>
        <Listbox
          id="reflection-voice"
          aria-labelledby="reflection-voice-label"
          className="mt-1"
          value={voice?.id ?? ''}
          options={
            made.length > 0
              ? made.map((item) => ({ value: item.id, label: item.name }))
              : [{ value: '', label: t('page.voice.noVoice') }]
          }
          disabled={made.length === 0}
          onChange={(value) => {
            setVoiceId(value)
            setPromptKey('')
          }}
        />
      </div>
      <div className="mt-6">
        <FieldLabel id="reflection-prompt-label" htmlFor="reflection-prompt">
          {t('page.voice.prompt')}
        </FieldLabel>
        <Listbox
          id="reflection-prompt"
          aria-labelledby="reflection-prompt-label"
          className="mt-1"
          value={prompt?.key ?? ''}
          options={
            answeredPrompts.length > 0
              ? answeredPrompts.map((item) => ({
                  value: item.key,
                  label: item.text,
                  disabled: item.photo && !allReadImages,
                }))
              : [{ value: '', label: t('page.voice.noAnswer') }]
          }
          disabled={answeredPrompts.length === 0}
          onChange={setPromptKey}
        />
        {answeredPrompts.some((item) => item.photo) && !allReadImages && (
          <Typography variant="label" as="p" className="mt-2">
            {t('page.voice.photoNeedsVision')}
          </Typography>
        )}
        {voice && !profilePending && answeredPrompts.length === 0 && (
          <Link
            to="/voices/$voiceId/materials"
            params={{ voiceId: voice.id }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover mt-2 inline-flex min-h-11 items-center underline',
            })}
          >
            {t('page.voice.answerPrompt')}
          </Link>
        )}
      </div>
      <div className="mt-6">
        <Button
          variant="cta"
          className="w-full sm:w-auto"
          pending={start.isPending}
          aria-disabled={!canStart || undefined}
          aria-describedby={reason ? hintId : undefined}
          onClick={() => void begin()}
        >
          {t('page.start')}
        </Button>
        <Typography variant="label" as="p" id={hintId} role="status" className="mt-2 empty:hidden">
          {reason}
        </Typography>
        {start.failure && (
          <Typography variant="body" as="div" role="alert" className="text-field-error mt-2">
            <AppFailureMessage failure={start.failure} />
          </Typography>
        )}
      </div>
    </>
  )
}
