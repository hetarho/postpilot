import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLatestWritingVoiceCandidates } from '@/entities/voice-candidate'
import { useLatestAuthoringSession } from '@/entities/ai-authoring'
import type { Voice } from '@/entities/voice'
import type { SetupController } from '@/features/complete-setup'
import {
  PrepareWritingVoice,
  LearningMaterials,
  type VoiceLearningNavigation,
} from '@/features/prepare-writing-voice'
import { VoiceQuestionnaire } from '@/features/answer-voice-prompt'
import { PasteMaterialForm } from '@/features/paste-voice-material'
import { GeneratedWritingVoices } from '@/features/generate-writing-voices'
import { AuthoringEditor, AuthoringPreview } from '@/features/ai-authoring'
import { Typography } from '@/shared/ui'
export function VoiceSetup({
  ownerId,
  controller,
  onNavigationChange,
}: {
  ownerId: string
  controller: SetupController
  onNavigationChange?: (navigation: VoiceLearningNavigation | null) => void
}) {
  const { t } = useTranslation('creation')
  const [initialVoiceId] = useState(
    () => controller.availability.voices.active.find((voice) => !voice.made)?.id ?? '',
  )
  const authoring = useLatestAuthoringSession({
    ownerId: initialVoiceId ? '' : ownerId,
    kind: 'writing-voice',
  })
  const legacy = useLatestWritingVoiceCandidates(initialVoiceId ? '' : ownerId)
  if (!initialVoiceId && (authoring.isPending || legacy.isPending))
    return (
      <Typography variant="body" role="status">
        {t('setup.checking')}
      </Typography>
    )
  const method = initialVoiceId
    ? 'questions'
    : authoring.data?.id
      ? 'ai'
      : legacy.batch?.jobId
        ? 'legacy'
        : 'choose'
  return (
    <ReadyVoiceSetup
      ownerId={ownerId}
      controller={controller}
      voiceId={initialVoiceId}
      method={method}
      onNavigationChange={onNavigationChange}
    />
  )
}
function ReadyVoiceSetup({
  ownerId,
  controller,
  voiceId,
  method,
  onNavigationChange,
}: {
  ownerId: string
  controller: SetupController
  voiceId: string
  method: 'choose' | 'questions' | 'ai' | 'legacy'
  onNavigationChange?: (navigation: VoiceLearningNavigation | null) => void
}) {
  const [initialMethod] = useState(method)
  const operation = useRef<number | null>(null)
  const busy = useCallback(
    (pending: boolean) => {
      if (pending && operation.current === null) {
        const started = controller.begin('voice')
        if (started !== null) {
          operation.current = started
          controller.running('voice', started)
        }
      } else if (!pending && operation.current !== null) {
        controller.success('voice', operation.current, false)
        operation.current = null
      }
    },
    [controller],
  )
  const complete = useCallback(
    (voice: Voice) => {
      if (!voice.made || voice.deleted || !voice.id) return
      const current = operation.current
      operation.current = null
      if (current !== null) controller.success('voice', current, true)
      else controller.next(true)
    },
    [controller],
  )
  return (
    <PrepareWritingVoice
      ownerId={ownerId}
      initialVoiceId={voiceId}
      initialMethod={initialMethod}
      voices={controller.availability.voices.active}
      onBusyChange={busy}
      onComplete={complete}
      onNavigationChange={onNavigationChange}
      renderQuestions={(props) => (
        <VoiceQuestionnaire
          ownerId={props.ownerId}
          voiceId={props.voiceId}
          samples={props.profile.samples}
          profile={props.profile}
          active={props.active}
          onBusyChange={props.onBusyChange}
          onReview={props.onReview}
          onNavigationChange={props.onNavigationChange}
          renderMakeVoice={() => null}
        />
      )}
      renderPaste={(props) => (
        <PasteMaterialForm
          ownerId={props.ownerId}
          voiceId={props.voiceId}
          value={props.pasteDraft}
          onChange={props.onPasteDraft}
          active={props.active}
          onBusyChange={props.onBusyChange}
          onSaved={props.onReview}
        />
      )}
      renderAI={(props) => (
        <AuthoringEditor
          ownerId={props.ownerId}
          kind="writing-voice"
          initialMakeDefault
          active={props.active}
          onNavigationChange={props.onNavigationChange}
          onBusyChange={props.onBusyChange}
          onSaved={(saved) => props.onConfirmed(saved.id)}
          renderPreview={(artifact) => (
            <AuthoringPreview kind="writing-voice" artifact={artifact} />
          )}
        />
      )}
      renderLegacy={(props) => (
        <GeneratedWritingVoices
          ownerId={props.ownerId}
          onBusyChange={props.onBusyChange}
          onAdopted={(voice) => props.onConfirmed(voice.id)}
        />
      )}
      renderMaterials={(props) => (
        <LearningMaterials
          ownerId={props.ownerId}
          voiceId={props.voiceId}
          samples={props.profile.samples}
        />
      )}
    />
  )
}
