import { useTranslation } from 'react-i18next'
import type { PostFingerprint } from '@/entities/voice'
import { Button, Typography } from '@/shared/ui'
import { FingerprintComparison } from './FingerprintComparison'

/** The post's voice reading (POST-102) as the head of 출처 보기's 말투 view (POST-128): this
 *  post's fingerprint beside its voice's, counted with no call. It carries no row chrome of its
 *  own; the page reads the data once and this draws it, or says the read failed with a retry. */
export function PostVoiceReading({
  fingerprint,
  isError,
  isFetching,
  onRetry,
}: {
  fingerprint: PostFingerprint | undefined
  isError: boolean
  isFetching: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  if (fingerprint)
    return (
      <FingerprintComparison
        items={fingerprint.items}
        textLabel={t('comparison.post.textLabel', { ns: 'voices' })}
      />
    )
  if (!isError) return null
  return (
    <Typography variant="meta" as="p" className="text-content-secondary mt-2">
      {t('comparison.post.failed', { ns: 'voices' })}{' '}
      <Button variant="ghost" onClick={onRetry} pending={isFetching}>
        {t('action.retry', { ns: 'common' })}
      </Button>
    </Typography>
  )
}
