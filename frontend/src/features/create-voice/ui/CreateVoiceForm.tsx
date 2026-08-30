import { useId, useState, type FormEvent } from 'react'
import { VOICE_NAME_MAX_CHARS } from '@/shared/config'
import { Button, FieldLabel, FieldMessage, TextField } from '@/shared/ui'
import { useCreateVoice } from '../api/useCreateVoice'

/** One field and the page's committing action. In flow, right after the field it commits — never
 *  docked: a text field inside a bottom bar sits behind the keyboard the moment it is focused
 *  (design-language §8.3). */
export function CreateVoiceForm({ ownerId, className }: { ownerId: string; className?: string }) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [name, setName] = useState('')
  const create = useCreateVoice(ownerId)
  // Counted the way the server counts (Unicode scalar values), so the field and the rule agree
  // on a name made of Hangul and emoji alike.
  const chars = Array.from(name.trim()).length
  const disabled = chars === 0 || chars > VOICE_NAME_MAX_CHARS || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    const submitted = name
    try {
      await create.create(submitted.trim())
      // Only what was submitted is cleared; a name typed during the round trip is kept.
      setName((current) => (current === submitted ? '' : current))
    } catch {
      // The mutation's message renders under the field.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className={className}>
      <FieldLabel htmlFor={id}>새 말투 이름</FieldLabel>
      <TextField
        id={id}
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder="예: 제품 리뷰"
        maxLength={VOICE_NAME_MAX_CHARS * 2}
        autoComplete="off"
        autoCapitalize="off"
        autoCorrect="off"
        enterKeyHint="done"
        aria-invalid={create.isError || undefined}
        aria-describedby={create.isError ? `${hintId} ${errorId}` : hintId}
        className="mt-1"
      />
      <p id={hintId} className="text-content-tertiary mt-2 text-xs">
        {chars} / {VOICE_NAME_MAX_CHARS}자 · 새 말투는 빈 프로필로 시작하고, 다른 말투와 아무것도
        공유하지 않아요.
      </p>
      {create.isError && (
        <FieldMessage id={errorId} className="mt-2">
          {create.errorMessage}
        </FieldMessage>
      )}
      <Button
        type="submit"
        variant="cta"
        disabled={disabled}
        pending={create.isPending}
        className="mt-4 w-full sm:w-auto"
      >
        말투 만들기
      </Button>
    </form>
  )
}
