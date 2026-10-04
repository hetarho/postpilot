import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { usePostFingerprint } from '@/entities/voice'
import { Button, Typography } from '@/shared/ui'
import { FingerprintComparison } from './FingerprintComparison'

/** ②'s 말투 지문 (POST-102): this post's fingerprint beside its voice's at the revision on
 *  screen, counted with no call. Nothing is drawn when the post has no comparison to show. */
export function PostFingerprintRow({
  ownerId,
  slug,
  revision,
  className,
}: {
  ownerId: string
  slug: string
  /** The content revision the row describes; a new one counts anew once autosaves stop moving
   *  it. */
  revision: bigint
  className?: string
}) {
  const { t } = useTranslation(['voices', 'common'])
  const headingId = useId()
  const { fingerprint, isError, isFetching, refetch } = usePostFingerprint(ownerId, slug, revision)
  // Counting makes no call and answers at once, so nothing holds the space while it does — and
  // a post with no comparison to show never shows a heading at all.
  if (fingerprint ? !fingerprint.applicable : !isError) return null
  return (
    <section aria-labelledby={headingId} className={className}>
      <Typography variant="label" as="h3" id={headingId}>
        {t('comparison.post.heading', { ns: 'voices' })}
      </Typography>
      {fingerprint ? (
        <FingerprintComparison
          items={fingerprint.items}
          textLabel={t('comparison.post.textLabel', { ns: 'voices' })}
        />
      ) : (
        <Typography variant="meta" as="p" className="text-content-secondary mt-2">
          {t('comparison.post.failed', { ns: 'voices' })}{' '}
          <Button variant="ghost" onClick={refetch} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Typography>
      )}
    </section>
  )
}
