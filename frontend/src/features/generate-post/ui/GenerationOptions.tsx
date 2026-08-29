import { useState } from 'react'
import { POST_TARGET_LENGTH_MAX, POST_TARGET_LENGTH_MIN } from '@/shared/config'
import { Button, Checkbox, FieldLabel, FieldMessage, Popover, TextField } from '@/shared/ui'
import { useGenerationOptions } from '../api/useGenerationOptions'

export function GenerationOptions({
  slug,
  targetLength,
  disabled,
  onSaved,
}: {
  slug: string
  targetLength?: number
  disabled: boolean
  onSaved: (value?: number) => void
}) {
  const save = useGenerationOptions()
  const [enabled, setEnabled] = useState(targetLength !== undefined)
  const [value, setValue] = useState(targetLength?.toString() ?? '')
  const parsed = Number(value)
  const valid =
    !enabled ||
    (Number.isInteger(parsed) &&
      parsed >= POST_TARGET_LENGTH_MIN &&
      parsed <= POST_TARGET_LENGTH_MAX)

  return (
    <Popover label="생성 옵션" disabled={disabled}>
      {(close) => (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            if (!valid) return
            const next = enabled ? parsed : undefined
            void save.save(slug, next).then(() => {
              onSaved(next)
              close()
            })
          }}
        >
          <label className="flex min-h-11 items-center gap-3 text-sm font-medium">
            <Checkbox checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
            목표 글자 수 사용
          </label>
          {enabled && (
            <div className="mt-3">
              <FieldLabel htmlFor={`generation-target-${slug}`}>목표 글자 수</FieldLabel>
              <TextField
                id={`generation-target-${slug}`}
                type="number"
                min={POST_TARGET_LENGTH_MIN}
                max={POST_TARGET_LENGTH_MAX}
                value={value}
                onChange={(event) => setValue(event.target.value)}
                aria-invalid={!valid || undefined}
                className="mt-1"
              />
              {!valid && (
                <FieldMessage className="mt-1">
                  {POST_TARGET_LENGTH_MIN.toLocaleString()}–
                  {POST_TARGET_LENGTH_MAX.toLocaleString()}자로 입력해 주세요.
                </FieldMessage>
              )}
            </div>
          )}
          {save.isError && <FieldMessage className="mt-2">옵션을 저장하지 못했어요.</FieldMessage>}
          <div className="mt-4 flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={close}>
              취소
            </Button>
            <Button type="submit" variant="secondary" disabled={!valid} pending={save.isPending}>
              저장
            </Button>
          </div>
        </form>
      )}
    </Popover>
  )
}
