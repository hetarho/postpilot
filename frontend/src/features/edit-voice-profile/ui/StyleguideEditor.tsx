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
      <h2 className="text-lg font-semibold tracking-tight">문체 규칙</h2>
      <p className="text-content-tertiary mt-1 text-xs">새 샘플을 학습하면 다시 작성됩니다.</p>
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
        // A generated styleguide is longer than any fixed box, and a fixed one would own every
        // vertical swipe that lands on it — the page is the only scroller (§4.4). `rows` is now
        // just the minimum height of an empty editor.
        rows={6}
        autoGrow
        aria-invalid={update.isError || undefined}
        aria-describedby={update.isError ? errorId : undefined}
        className="mt-1 leading-relaxed"
      />
      {update.isError && (
        <FieldMessage id={errorId} className="mt-2">
          {update.errorMessage}
        </FieldMessage>
      )}
      {/* 저장 goes AFTER the field it commits (§4.3). In the section header it sat ~377px above
          the bottom of the styleguide — off the top of the screen exactly when the keyboard is up
          and the user is typing into it, and out of thumb reach even when it is not. There is no
          autosave here, so an unreachable 저장 is a lost edit. */}
      <div className="mt-4 flex justify-end">
        <Button
          variant="secondary"
          onClick={() => void save()}
          disabled={!dirty}
          pending={update.isSaving}
          className="w-full sm:w-auto"
        >
          저장
        </Button>
      </div>
      <span role="status" className="sr-only">
        {update.isSaving ? '문체 규칙을 저장하는 중' : ''}
      </span>
    </section>
  )
}
