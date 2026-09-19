import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Textarea, Typography, typographyStyles } from '@/shared/ui'

/** The 가제 belongs to 글 생성 alone (policy/posts.md). From 글 다듬기 on there is exactly one
 *  title on the screen and it is `content.title`, edited through the block editor's header — two
 *  title fields side by side is a question the user cannot answer.
 *
 *  A textarea, not an input: a Korean title fits ~14 characters across a 360px screen at the
 *  display size, and a single-line input would scroll the rest of it out of a field that has no
 *  well to show it scrolled (§0 — the title is one of the largest things on the screen, so it
 *  wraps instead). Its value and its autosave stay in `DraftEditor`, so unmounting it on another
 *  step cannot strand a queued save. */
export function TitleField({
  value,
  onChange,
  fieldRef,
  nextRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
  nextRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-title" className="sr-only">
        {t('editor.title')}
      </FieldLabel>
      {/* The visible title remains an editable field, so this mirrors its current value into the
          document outline without replacing the field or creating a second tab stop. */}
      <Typography variant="display" className="sr-only">
        {value.trim() || t('editor.titlePlaceholder')}
      </Typography>
      <Textarea
        id="post-title"
        ref={fieldRef}
        appearance="bare"
        rows={1}
        autoGrow
        value={value}
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
        placeholder={t('editor.titlePlaceholder')}
        enterKeyHint="next"
        autoCapitalize="off"
        autoComplete="off"
        // The bare editor's caller owns the field's type (§7): the title wears the display role —
        // the post title is the largest thing on the screen (§0).
        className={typographyStyles({ variant: 'display', className: 'mt-4' })}
      />
    </>
  )
}
