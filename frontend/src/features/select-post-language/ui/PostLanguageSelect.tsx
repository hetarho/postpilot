import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect, type AppFailure, type ContentLanguage } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { FieldLabel, FieldMessage, Listbox, Typography } from '@/shared/ui'

export function PostLanguageSelect({
  value,
  contentLanguage,
  frozenLanguage,
  onSelect,
  className,
}: {
  value: ContentLanguage
  contentLanguage?: ContentLanguage
  frozenLanguage?: ContentLanguage
  onSelect: (language: ContentLanguage) => Promise<void> | void
  className?: string
}) {
  const { t } = useTranslation(['posts', 'common'])
  const id = useId()
  const labelId = `${id}-label`
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  const select = async (language: ContentLanguage) => {
    setFailure(undefined)
    setPending(true)
    try {
      await onSelect(language)
    } catch (error) {
      setFailure(appFailureFromConnect(error))
    } finally {
      setPending(false)
    }
  }
  return (
    <div className={className}>
      <FieldLabel id={labelId} htmlFor={id}>
        {t('editor.language.label', { ns: 'posts' })}
      </FieldLabel>
      {/* A `Listbox` even though two fixed options would also fit a `SegmentedControl` (§7): this
          field sits in the 글쓰기 옵션 brief, and the five controls there read as ONE brief only
          while they wear the same field well. */}
      <Listbox<ContentLanguage>
        id={id}
        aria-labelledby={labelId}
        value={value}
        options={[
          { value: 'ko', label: t('contentLanguage.ko', { ns: 'common' }) },
          { value: 'en', label: t('contentLanguage.en', { ns: 'common' }) },
        ]}
        disabled={pending}
        aria-invalid={failure ? true : undefined}
        aria-describedby={`${hintId}${failure ? ` ${errorId}` : ''}`}
        onChange={(next) => void select(next)}
        className="mt-1"
      />
      <Typography variant="body" as="p" id={hintId} className="text-content-secondary mt-2">
        {frozenLanguage && frozenLanguage !== value
          ? t('editor.language.frozen', {
              ns: 'posts',
              language: t(`contentLanguage.${frozenLanguage}`, { ns: 'common' }),
            })
          : contentLanguage && contentLanguage !== value
            ? t('editor.language.mismatch', {
                ns: 'posts',
                language: t(`contentLanguage.${contentLanguage}`, { ns: 'common' }),
              })
            : t('editor.language.help', { ns: 'posts' })}
      </Typography>
      {failure && (
        <FieldMessage id={errorId} className="mt-2">
          {formatAppFailure(failure)}
        </FieldMessage>
      )}
    </div>
  )
}
