import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Textarea } from '@/shared/ui'

/** The 가제 belongs to 글 생성 alone (POST-50). From 글 다듬기 on there is exactly one
 *  title on the screen and it is `content.title`, edited through the block editor's header — two
 *  title fields side by side is a question the user cannot answer.
 *
 *  It is the first field of 내용, so it is drawn as one: a named well, like the template answers
 *  under it. As bare display-size text with its name hidden it sat under the screen's question as a
 *  second, larger heading, and nothing said it could be typed in.
 *
 *  A textarea, not an input: a Korean title fits only a short line across a 360px screen, and a
 *  single-line input would scroll the rest of it out of sight, so it wraps instead. Its value and
 *  its autosave stay in `DraftEditor`, so unmounting it on another step cannot strand a queued
 *  save. */
export function TitleField({
  value,
  onChange,
  fieldRef,
  nextRef,
  readOnly = false,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
  nextRef: RefObject<HTMLTextAreaElement | null>
  /** A published post's 가제: readable and selectable, never edited (POST-86). */
  readOnly?: boolean
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-title" className="mt-4 flex min-h-11 items-center">
        {t('editor.title')}
      </FieldLabel>
      <Textarea
        id="post-title"
        ref={fieldRef}
        rows={1}
        autoGrow
        value={value}
        readOnly={readOnly}
        // A pasted newline would otherwise be saved inside the title; the single-line input this
        // replaced dropped one for free.
        onChange={(event) => onChange(event.target.value.replace(/\n/g, ' '))}
        onKeyDown={(event) => {
          // The title is one line even though the field wraps, so Enter moves to the memo instead
          // of typing a newline into it — but never mid-composition, where Enter is how a Hangul
          // IME commits the syllable being written.
          if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
          event.preventDefault()
          nextRef.current?.focus()
        }}
        placeholder={t('guide.titlePlaceholder')}
        enterKeyHint="next"
        autoCapitalize="off"
        autoComplete="off"
        className="mt-1"
      />
    </>
  )
}
