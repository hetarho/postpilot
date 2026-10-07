import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type VoiceSample,
  type VoiceSampleDetail,
  type VoicePrompt,
  useDeleteVoiceSample,
  useVoicePrompts,
  useVoiceSample,
} from '@/entities/voice'
import { formatRelativeTime } from '@/shared/lib'
import { Badge, Button, Dialog, FieldMessage, Sheet, Typography } from '@/shared/ui'

/** The voice's 학습 글, newest first (VOICE-64): a pasted post by its label, an answer by its
 *  prompt, each opening to its full text and photo with `삭제`. */
export interface SampleEditorProps {
  ownerId: string
  voiceId: string
  detail: VoiceSampleDetail
  prompt?: VoicePrompt
  blocked: boolean
  onSaved: () => void
  onCancel: () => void
  onBusyChange: (busy: boolean) => void
}
interface SampleListProps {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  blocked?: boolean
  renderEditor?: (props: SampleEditorProps) => ReactNode
}
export function SampleList(props: SampleListProps) {
  return <OwnedSampleList key={JSON.stringify([props.ownerId, props.voiceId])} {...props} />
}
function OwnedSampleList({
  ownerId,
  voiceId,
  samples,
  blocked = false,
  renderEditor,
}: SampleListProps) {
  const { t } = useTranslation('voices')
  const { prompts } = useVoicePrompts()
  const [opened, setOpened] = useState<VoiceSample | null>(null)
  const titleOf = (sample: VoiceSample) =>
    sample.kind === 'answer'
      ? (prompts.find((prompt) => prompt.key === sample.promptKey)?.text ?? '')
      : sample.label

  return (
    // The tab's own heading names the list (학습 글), so it carries none of its own.
    <section aria-label={t('samples.title')}>
      {samples.length === 0 ? (
        <Typography variant="body" className="text-content-tertiary">
          {t('samples.empty')}
        </Typography>
      ) : (
        <ul className="divide-divider divide-y">
          {samples.map((sample) => (
            <li key={sample.id}>
              <button
                type="button"
                onClick={() => setOpened(sample)}
                className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-14 w-full items-center gap-3 py-2 text-left"
              >
                <span className="min-w-0 flex-1">
                  <Typography variant="body" as="span" className="block truncate">
                    {titleOf(sample)}
                  </Typography>
                  <Typography variant="meta" className="mt-1 block">
                    {t(sample.kind === 'answer' ? 'samples.answer' : 'samples.post')} ·{' '}
                    {formatRelativeTime(sample.createdAt)}
                  </Typography>
                </span>
                {sample.hasPhoto && <Badge>{t('samples.photo')}</Badge>}
              </button>
            </li>
          ))}
        </ul>
      )}
      {opened && (
        <SampleSheet
          ownerId={ownerId}
          voiceId={voiceId}
          sample={opened}
          title={titleOf(opened)}
          blocked={blocked}
          prompt={prompts.find((prompt) => prompt.key === opened.promptKey)}
          renderEditor={renderEditor}
          onClose={() => setOpened(null)}
        />
      )}
    </section>
  )
}

function SampleSheet({
  ownerId,
  voiceId,
  sample,
  title,
  blocked,
  prompt,
  renderEditor,
  onClose,
}: {
  ownerId: string
  voiceId: string
  sample: VoiceSample
  title: string
  blocked: boolean
  prompt?: VoicePrompt
  renderEditor?: (props: SampleEditorProps) => ReactNode
  onClose: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const titleId = useId()
  const { detail, isError, refetch } = useVoiceSample(ownerId, voiceId, sample.id)
  const remove = useDeleteVoiceSample(ownerId, voiceId)
  const [confirming, setConfirming] = useState(false)
  const [editing, setEditing] = useState(false)
  const [editBusy, setEditBusy] = useState(false)

  const confirm = async () => {
    try {
      await remove.remove(sample.id)
      onClose()
    } catch {
      // The mutation's message renders in the sheet.
    } finally {
      setConfirming(false)
    }
  }

  return (
    <Sheet
      open
      labelledBy={titleId}
      onClose={() => {
        if (!editBusy) onClose()
      }}
    >
      <Typography variant="title" as="h2" id={titleId} className="break-words">
        {detail?.sample.kind === 'post' ? detail.sample.label : title}
      </Typography>
      {editing && detail && renderEditor ? (
        renderEditor({
          ownerId,
          voiceId,
          detail,
          prompt,
          blocked,
          onSaved: () => {
            setEditing(false)
            refetch()
          },
          onCancel: () => setEditing(false),
          onBusyChange: setEditBusy,
        })
      ) : (
        <>
          {isError ? (
            <FieldMessage className="mt-4">
              {t('samples.loadFailed', { ns: 'voices' })}
            </FieldMessage>
          ) : detail ? (
            <>
              {detail.photoUrl && (
                <img
                  src={detail.photoUrl}
                  width={detail.photoWidth}
                  height={detail.photoHeight}
                  alt={t('samples.photoAlt', { ns: 'voices' })}
                  className="max-h-field mt-4 h-auto w-full rounded-md object-contain"
                />
              )}
              <Typography variant="body" as="p" className="mt-4 break-words whitespace-pre-line">
                {detail.body}
              </Typography>
            </>
          ) : (
            <Typography variant="body" role="status" className="text-content-tertiary mt-4">
              {t('state.loading', { ns: 'common' })}
            </Typography>
          )}
          {remove.isError && <FieldMessage className="mt-3">{remove.errorMessage}</FieldMessage>}
          <div className="mt-6 flex flex-wrap justify-end gap-2">
            <Button variant="ghost" onClick={onClose}>
              {t('action.close', { ns: 'common' })}
            </Button>
            {renderEditor && (
              <Button
                variant="secondary"
                disabled={blocked || !detail || remove.isPending}
                onClick={() => setEditing(true)}
              >
                {t('samples.edit', { ns: 'voices' })}
              </Button>
            )}
            <Button
              variant="danger"
              disabled={blocked || remove.isPending}
              onClick={() => setConfirming(true)}
            >
              {t('action.delete', { ns: 'common' })}
            </Button>
          </div>
        </>
      )}
      <Dialog
        open={confirming}
        title={t('samples.deleteTitle', { ns: 'voices' })}
        confirmLabel={t('action.delete', { ns: 'common' })}
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void confirm()}
      >
        {t('samples.deleteDescription', { ns: 'voices' })}
      </Dialog>
    </Sheet>
  )
}
