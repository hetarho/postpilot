import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect, type AppFailure, type ContentLanguage } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { FieldLabel, FieldMessage, Select, Typography } from '@/shared/ui'

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
      <FieldLabel htmlFor={id}>{t('editor.language.label', { ns: 'posts' })}</FieldLabel>
      <Select
        id={id}
        value={value}
        disabled={pending}
        aria-invalid={failure ? true : undefined}
        aria-describedby={`${hintId}${failure ? ` ${errorId}` : ''}`}
        onChange={(event) => void select(event.target.value as ContentLanguage)}
        className="mt-1"
      >
        <option value="ko">{t('contentLanguage.ko', { ns: 'common' })}</option>
        <option value="en">{t('contentLanguage.en', { ns: 'common' })}</option>
      </Select>
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
