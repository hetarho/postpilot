import { useId, useRef, useState, type ReactNode, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipChannel,
  CLIP_CHANNELS,
  ClipFailureNotice,
  ClipQuoteApproval,
  useClipUploadTexts,
  type ClipChannel,
  type ClipProject,
  type ClipUploadText as UploadText,
  type ClipUploadTextLimits,
} from '@/entities/clip-project'
import { type ModelRef, progressLabel } from '@/entities/generation-job'
import { useMyPlan } from '@/entities/plan'
import { appFailureFromConnect } from '@/shared/api'
import { Button, Dialog, Textarea, TextField, Typography } from '@/shared/ui'
import { uploadCopyText, uploadTags, uploadTagText, uploadTextFits } from '../model/text'
import { useTextCopy } from '../model/useTextCopy'
import { useUploadTextAction } from '../model/useUploadTextAction'

export interface ClipUploadTextProps {
  ownerId: string
  project: ClipProject
  writeModel: ModelRef | null
  modelSelect?: ReactNode
  disabled?: boolean
  beforeWrite?: () => Promise<unknown>
}
export function ClipUploadText(props: ClipUploadTextProps) {
  return <UploadTextPanel key={`${props.ownerId}/${props.project.id}`} {...props} />
}
function UploadTextPanel(props: ClipUploadTextProps) {
  const { t } = useTranslation('clips')
  const { ownerId, project } = props
  const [editing, setEditing] = useState(false)
  const [channel, setChannel] = useState<ClipChannel>(clipChannel(project.channel))
  const query = useClipUploadTexts(ownerId, project.id)
  const queue = useRef<Promise<void>>(Promise.resolve())
  const row = query.data?.texts.find((text) => text.channel === channel)
  const limits = query.data?.limits.find((item) => item.channel === channel)
  return (
    <section aria-label={t('uploadText.title')} className="min-w-0 space-y-3">
      <Typography variant="title" as="h2">
        {t('uploadText.title')}
      </Typography>
      <Typography variant="body">{t('uploadText.help')}</Typography>
      <div className="flex flex-wrap gap-2" role="group" aria-label={t('project.channel')}>
        {CLIP_CHANNELS.map((value) => (
          <Button
            key={value}
            variant={value === channel ? 'secondary' : 'ghost'}
            aria-pressed={value === channel}
            disabled={editing && value !== channel}
            onClick={() => setChannel(value)}
          >
            {t(`channel.${value}`)}
          </Button>
        ))}
      </div>
      {query.isPending && (
        <Typography role="status" variant="meta">
          {t('uploadText.checking')}
        </Typography>
      )}
      {query.isError && (
        <>
          <ClipFailureNotice failure={appFailureFromConnect(query.error)} />
          <Button variant="secondary" onClick={() => void query.refetch()}>
            {t('uploadText.retry')}
          </Button>
        </>
      )}
      {limits && (
        <ChannelText
          key={channel}
          {...props}
          channel={channel}
          row={row}
          limits={limits}
          queue={queue}
          onEditingChange={setEditing}
        />
      )}
    </section>
  )
}
function ChannelText({
  ownerId,
  project,
  channel,
  row,
  limits,
  queue,
  writeModel,
  modelSelect,
  disabled,
  beforeWrite,
  onEditingChange,
}: ClipUploadTextProps & {
  channel: ClipChannel
  row?: UploadText
  limits: ClipUploadTextLimits
  queue: RefObject<Promise<void>>
  onEditingChange: (editing: boolean) => void
}) {
  const { t } = useTranslation('clips')
  const { myPlan } = useMyPlan()
  const action = useUploadTextAction({
    ownerId,
    project,
    channel,
    writeModel,
    revision: row?.revision ?? 0,
    beforeWrite,
  })
  const [confirm, setConfirm] = useState(false)
  const [draft, setDraft] = useState<{ body: string; tags: string; revision: number }>()
  const bodyId = useId(),
    tagsId = useId()
  const busy = !!disabled || action.busy
  const canonicalBody = row ? uploadCopyText(project, row.body) : ''
  const tags = draft ? uploadTags(draft.tags) : (row?.tags ?? [])
  const body = draft ? uploadCopyText(project, draft.body.trim()) : canonicalBody
  const fits = !!(draft ? draft.body : row?.body)?.trim() && uploadTextFits(body, tags, limits)
  const used = Array.from(
    body + (limits.tagsInCaption && tags.length ? `\n\n${uploadTagText(tags)}` : ''),
  ).length
  const request = () => {
    if (busy || draft) return
    if (row?.edited) setConfirm(true)
    else void action.prepare(false)
  }
  return (
    <div className="min-w-0 space-y-3">
      {row ? (
        draft ? (
          <div className="space-y-2">
            <Typography variant="fieldTitle" as="label" htmlFor={bodyId}>
              {t('uploadText.body')}
            </Typography>
            <Textarea
              id={bodyId}
              value={draft.body}
              rows={6}
              disabled={busy}
              onChange={(event) => setDraft({ ...draft, body: event.target.value })}
            />
            <Typography variant="fieldTitle" as="label" htmlFor={tagsId}>
              {t('uploadText.tags')}
            </Typography>
            <TextField
              id={tagsId}
              value={draft.tags}
              disabled={busy}
              onChange={(event) => setDraft({ ...draft, tags: event.target.value })}
            />
            <Typography variant="meta">
              {t('uploadText.count', {
                used,
                max: limits.maxChars,
                tags: tags.length,
                maxTags: limits.maxTags,
              })}
            </Typography>
            {!fits && (
              <Typography variant="meta" role="alert">
                {t('uploadText.limit')}
              </Typography>
            )}
            <div className="flex flex-wrap gap-2">
              <Button
                variant="cta"
                disabled={busy || !fits}
                pending={action.phase === 'saving'}
                onClick={() => {
                  void action.save(draft.body, tags, draft.revision).then((saved) => {
                    if (saved) {
                      setDraft(undefined)
                      onEditingChange(false)
                    }
                  })
                }}
              >
                {t('uploadText.save')}
              </Button>
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => {
                  setDraft(undefined)
                  onEditingChange(false)
                }}
              >
                {t('uploadText.cancelEdit')}
              </Button>
            </div>
          </div>
        ) : (
          <>
            <TextOutput
              key={JSON.stringify([channel, row.revision, canonicalBody, row.tags])}
              body={canonicalBody}
              tags={row.tags}
              limits={limits}
              queue={queue}
            />
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => {
                action.dismiss()
                onEditingChange(true)
                setDraft({ body: row.body, tags: uploadTagText(row.tags), revision: row.revision })
              }}
            >
              {t('uploadText.edit')}
            </Button>
          </>
        )
      ) : (
        <Typography variant="body">{t('uploadText.empty')}</Typography>
      )}
      {draft && <Typography variant="meta">{t('uploadText.unsaved')}</Typography>}
      {action.running ? (
        <div className="space-y-2">
          <Typography variant="body" role="status">
            {action.job ? progressLabel(action.job) : t('uploadText.running')}
          </Typography>
          <Button
            variant="secondary"
            disabled={!!action.job?.cancelRequestedAt || action.job?.canCancel === false}
            onClick={() => void action.cancel()}
          >
            {t('uploadText.cancel')}
          </Button>
        </div>
      ) : (
        <>
          {modelSelect}
          <Button
            variant="secondary"
            disabled={busy || !!draft || !writeModel}
            pending={action.phase === 'quoting' || action.phase === 'starting'}
            onClick={request}
          >
            {row
              ? t('uploadText.rewrite')
              : channel === project.channel
                ? t('uploadText.make')
                : t(`uploadText.other.${channel}`)}
          </Button>
        </>
      )}
      {action.quote && !draft && !action.running && (
        <ClipQuoteApproval
          quote={action.quote}
          quoting={action.phase === 'quoting' || action.phase === 'starting'}
          expired={action.expired}
          balance={myPlan?.balance}
          approveDisabled={busy || !myPlan || action.expired}
          approveLabel={t('uploadText.approve', { amount: action.quote.maxCredits })}
          onRefresh={action.refresh}
          onApprove={(quote) => void action.start(quote)}
        />
      )}
      <ClipFailureNotice failure={action.failure} />
      <Dialog
        open={confirm}
        title={t('uploadText.confirmTitle')}
        confirmLabel={t('uploadText.confirm')}
        onClose={() => setConfirm(false)}
        onConfirm={() => {
          setConfirm(false)
          void action.prepare(true)
        }}
      >
        {t('uploadText.confirmHelp')}
      </Dialog>
    </div>
  )
}
function TextOutput({
  body,
  tags,
  limits,
  queue,
}: {
  body: string
  tags: string[]
  limits: ClipUploadTextLimits
  queue: RefObject<Promise<void>>
}) {
  const { t } = useTranslation('clips')
  const { copy, copied, manual, fieldRef } = useTextCopy(queue)
  const fits = uploadTextFits(body, tags, limits)
  return (
    <div className="space-y-2">
      <Typography variant="body" as="p" className="break-words whitespace-pre-wrap">
        {body}
      </Typography>
      <Typography variant="body" as="p" className="break-words">
        {uploadTagText(tags)}
      </Typography>
      <Typography variant="meta">
        {t('uploadText.count', {
          used: Array.from(
            body + (limits.tagsInCaption && tags.length ? `\n\n${uploadTagText(tags)}` : ''),
          ).length,
          max: limits.maxChars,
          tags: tags.length,
          maxTags: limits.maxTags,
        })}
      </Typography>
      {!fits && (
        <Typography variant="meta" role="alert">
          {t('uploadText.limit')}
        </Typography>
      )}
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" disabled={!fits} onClick={() => void copy('body', body)}>
          {t('uploadText.copyBody')}
        </Button>
        <Button
          variant="secondary"
          disabled={!tags.length || !fits}
          onClick={() => void copy('tags', uploadTagText(tags))}
        >
          {t('uploadText.copyTags')}
        </Button>
        {limits.tagsInCaption && (
          <Button
            variant="secondary"
            disabled={!fits}
            onClick={() =>
              void copy('all', body + (tags.length ? `\n\n${uploadTagText(tags)}` : ''))
            }
          >
            {t('uploadText.copyAll')}
          </Button>
        )}
      </div>
      <Typography variant="meta" role="status" aria-live="polite">
        {copied ? t('uploadText.copied') : manual ? t('uploadText.manual') : ''}
      </Typography>
      {manual && (
        <Textarea
          ref={fieldRef}
          readOnly
          rows={5}
          aria-label={t('uploadText.manualField')}
          value={manual.value}
        />
      )}
    </div>
  )
}
