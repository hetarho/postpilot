import { useSearch } from '@tanstack/react-router'
import { CreationSetup } from '@/widgets/creation-setup'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'

/** Setup's template questions are answered by talking: the same conversation that makes a
 *  template from 내 스타일, one AI draft refined by talking, saved with its own 저장. */
export function SetupPage() {
  const { restart } = useSearch({ from: '/authenticated/setup' })
  return (
    <CreationSetup
      restart={restart}
      renderTemplate={({ ownerId, kind, onSaved }) => (
        <AIAuthoringStudio
          layout="conversation"
          ownerId={ownerId}
          kind={kind}
          candidateCount={1}
          startFromSaved
          onSaved={onSaved}
          className="flex-1"
        />
      )}
    />
  )
}
