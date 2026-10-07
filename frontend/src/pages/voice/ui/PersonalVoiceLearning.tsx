import { useNavigate } from '@tanstack/react-router'
import { PrepareWritingVoice, LearningMaterials } from '@/features/prepare-writing-voice'
import { VoiceQuestionnaire } from '@/features/answer-voice-prompt'
import { EditVoiceMaterial } from '@/features/edit-voice-material'
import { PasteMaterialForm } from '@/features/paste-voice-material'
import { AuthoringEditor, AuthoringPreview } from '@/features/ai-authoring'
export function PersonalVoiceLearning({
  ownerId,
  voiceId,
  onComplete,
}: {
  ownerId: string
  voiceId: string
  onComplete?: () => void
}) {
  const navigate = useNavigate()
  return (
    <PrepareWritingVoice
      ownerId={ownerId}
      initialVoiceId={voiceId}
      onComplete={(voice) => {
        onComplete?.()
        void navigate({ to: '/voices/$voiceId', params: { voiceId: voice.id } })
      }}
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
          active={props.active}
          onNavigationChange={props.onNavigationChange}
          onBusyChange={props.onBusyChange}
          onSaved={(saved) => props.onConfirmed(saved.id)}
          renderPreview={(artifact) => (
            <AuthoringPreview kind="writing-voice" artifact={artifact} />
          )}
        />
      )}
      renderMaterials={(props) => (
        <LearningMaterials
          ownerId={props.ownerId}
          voiceId={props.voiceId}
          samples={props.profile.samples}
          blocked={props.profile.voice.deleted}
          onBusyChange={props.onBusyChange}
          renderEditor={(editor) => <EditVoiceMaterial {...editor} />}
        />
      )}
    />
  )
}
