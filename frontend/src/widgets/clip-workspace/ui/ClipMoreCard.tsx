import { useId, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  clipChannel,
  clipConfirmed,
  CLIP_CHANNELS,
  type ClipChannel,
  type ClipProject,
} from '@/entities/clip-project'
import { useSavePostDraft } from '@/entities/post'
import { useVoices } from '@/entities/voice'
import { activeLocale } from '@/shared/lib/localization'
import { Button, buttonStyles, cardStyles, Notice, Typography } from '@/shared/ui'
import { createPostFromClip } from '../model/createPostFromClip'

/** ③'s one 더 해 보기 card (CLIP-233): a template from the clip's form, a post from the same
 *  visit, and the other clip channel's 업로드 문구. Enabled while the plan is confirmed or the
 *  clip finalized; none of it starts AI work or changes the clip. */
export function ClipMoreCard({
  ownerId,
  project,
  onOtherChannel,
}: {
  ownerId: string
  project: ClipProject
  /** Opens the other channel's 업로드 문구 where it stands in ③ (CLIP-211). */
  onOtherChannel: (channel: ClipChannel) => void
}) {
  const { t } = useTranslation('clips')
  const headingId = useId()
  const navigate = useNavigate()
  const { save } = useSavePostDraft()
  const { defaultVoice } = useVoices(ownerId)
  const [creating, setCreating] = useState(false)
  const [failed, setFailed] = useState(false)
  const enabled = clipConfirmed(project) || !!project.finalized
  const channel = clipChannel(project.channel)
  const other = CLIP_CHANNELS.find((value) => value !== channel) ?? channel
  const writePost = () => {
    if (creating) return
    setCreating(true)
    setFailed(false)
    createPostFromClip({
      project,
      fallbackLanguage: activeLocale(),
      voiceId: defaultVoice?.made ? defaultVoice.id : undefined,
      save,
    }).then(
      (slug) => void navigate({ to: '/posts/$slug', params: { slug } }),
      () => {
        setCreating(false)
        setFailed(true)
      },
    )
  }
  return (
    <section aria-labelledby={headingId} className={cardStyles({ className: 'mt-10' })}>
      <Typography variant="fieldTitle" as="h2" id={headingId}>
        {t('more.title')}
      </Typography>
      {!enabled && (
        <Typography variant="meta" as="p" className="mt-1">
          {t('more.waiting')}
        </Typography>
      )}
      <div className="mt-4 flex flex-wrap items-start gap-2">
        {enabled ? (
          <Link
            to="/video-templates/new"
            search={{ channel, from: project.id }}
            className={buttonStyles({ variant: 'secondary' })}
          >
            {t('more.template')}
          </Link>
        ) : (
          <Button variant="secondary" disabled>
            {t('more.template')}
          </Button>
        )}
        <Button variant="secondary" disabled={!enabled} pending={creating} onClick={writePost}>
          {t('more.post')}
        </Button>
        <Button variant="secondary" disabled={!enabled} onClick={() => onOtherChannel(other)}>
          {t(`more.other.${other}`)}
        </Button>
      </div>
      {failed && (
        <Notice tone="danger" role="alert" className="mt-3">
          {t('more.postFailed')}
        </Notice>
      )}
    </section>
  )
}
