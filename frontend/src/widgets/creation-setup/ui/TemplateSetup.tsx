import { AuthoringEditor, AuthoringPreview } from '@/features/ai-authoring'
import { TemplateDirectEditor } from '@/features/edit-template'
import { ClipTemplateDirectEditor } from '@/features/edit-clip-template'
import type { SetupController } from '@/features/complete-setup'
import { templateIllustration } from '@/shared/assets'

/** Guide: setup's template question is the same conversation 템플릿 and 영상 템플릿 open — the
 *  owner says what they post, the AI drafts one template from scratch, they refine it by talking
 *  and save. Saving it is the step's one canonical save, so it moves setup on; until then the
 *  way back and the skip stay on setup's own step bar. Nothing runs before the owner sends, and
 *  every send is quoted first (the conversation's own estimate). */
export function TemplateSetup({
  ownerId,
  kind,
  controller,
}: {
  ownerId: string
  kind: 'post-template' | 'clip-template'
  controller: SetupController
}) {
  const authoringKind = kind === 'post-template' ? 'post-template' : 'video-template'
  return (
    <AuthoringEditor
      key={ownerId}
      ownerId={ownerId}
      kind={authoringKind}
      layout="conversation"
      candidateCount={1}
      initialMethod="ai"
      startFromSaved
      draftIllustration={templateIllustration}
      renderDirectEditor={(props) =>
        kind === 'post-template' ? (
          <TemplateDirectEditor {...props} />
        ) : (
          <ClipTemplateDirectEditor {...props} />
        )
      }
      renderPreview={(artifact, lastValid) => (
        <AuthoringPreview kind={authoringKind} artifact={artifact} fallbackArtifact={lastValid} />
      )}
      onSaved={() => {
        const operation = controller.begin(kind)
        if (operation !== null) controller.success(kind, operation, true)
      }}
    />
  )
}
