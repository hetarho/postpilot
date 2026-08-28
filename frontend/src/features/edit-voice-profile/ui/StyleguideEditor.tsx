import { useId, useState } from 'react'
import { useUpdateVoiceProfile } from '@/entities/voice-profile'
import { Button, FieldLabel, FieldMessage, Textarea } from '@/shared/ui'

export function StyleguideEditor({ ownerId, styleguide }: { ownerId: string; styleguide: string }) {
  const id = useId()
  const errorId = `${id}-error`
  const [draft, setDraft] = useState<string | null>(null)
  const update = useUpdateVoiceProfile(ownerId)
  const value = draft ?? styleguide
  const dirty = draft !== null

  const save = async () => {
    const submitted = value
    try {
      await update.saveStyleguide(submitted)
      setDraft((current) => (current === submitted ? null : current))
    } catch {
      // The mutation error is rendered below.
    }
  }

  return (
    <section>
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">문체 규칙</h2>
          <p className="text-content-tertiary mt-1 text-xs">새 샘플을 학습하면 다시 작성됩니다.</p>
        </div>
        <Button
          variant="secondary"
          onClick={() => void save()}
          disabled={!dirty || update.isSaving}
        >
          {update.isSaving ? '저장 중…' : '저장'}
        </Button>
      </div>
      <FieldLabel htmlFor={id} className="mt-4">
        문체 규칙
      </FieldLabel>
      <Textarea
        id={id}
        value={value}
        onChange={(event) => {
          const next = event.target.value
          setDraft(next === styleguide ? null : next)
        }}
        rows={14}
        aria-invalid={update.isError || undefined}
        aria-describedby={update.isError ? errorId : undefined}
        className="mt-1 leading-relaxed"
      />
      {update.isError && (
        <FieldMessage id={errorId} className="mt-2">
          {update.errorMessage}
        </FieldMessage>
      )}
      <span role="status" className="sr-only">
        {update.isSaving ? '문체 규칙을 저장하는 중' : ''}
      </span>
    </section>
  )
}
