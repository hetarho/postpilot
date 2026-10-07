import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useVoicePrompts,
  useVoiceSample,
  type VoiceSample,
  type VoiceSampleDetail,
  type VoicePrompt,
} from '@/entities/voice'
import { Button, Notice, Sheet, Typography } from '@/shared/ui'
export function LearningMaterials({
  ownerId,
  voiceId,
  samples,
  blocked = false,
  renderEditor,
  onBusyChange,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  blocked?: boolean
  onBusyChange?: (busy: boolean) => void
  renderEditor?: (props: {
    ownerId: string
    voiceId: string
    detail: VoiceSampleDetail
    prompt?: VoicePrompt
    blocked: boolean
    onSaved: () => void
    onCancel: () => void
    onBusyChange: (busy: boolean) => void
  }) => ReactNode
}) {
  const { t } = useTranslation('voicePreparation')
  const { prompts } = useVoicePrompts()
  const [opened, setOpened] = useState<VoiceSample | null>(null)
  const title = (sample: VoiceSample) =>
    sample.label ||
    prompts.find((prompt) => prompt.key === sample.promptKey)?.text ||
    t('savedMaterial')
  return (
    <>
      <ul className="divide-divider divide-y">
        {samples.map((sample) => (
          <li key={sample.id}>
            <Button
              variant="ghost"
              className="w-full justify-start py-3 text-left"
              onClick={() => setOpened(sample)}
            >
              {title(sample)}
            </Button>
          </li>
        ))}
      </ul>
      {opened && (
        <ReadMaterial
          key={JSON.stringify([ownerId, voiceId, opened.id])}
          ownerId={ownerId}
          voiceId={voiceId}
          sampleId={opened.id}
          title={title(opened)}
          onClose={() => setOpened(null)}
          blocked={blocked}
          prompt={prompts.find((prompt) => prompt.key === opened.promptKey)}
          renderEditor={renderEditor}
          onBusyChange={onBusyChange}
        />
      )}
    </>
  )
}
function ReadMaterial({
  ownerId,
  voiceId,
  sampleId,
  title,
  onClose,
  blocked,
  prompt,
  renderEditor,
  onBusyChange,
}: {
  ownerId: string
  voiceId: string
  sampleId: string
  title: string
  onClose: () => void
  blocked: boolean
  onBusyChange?: (busy: boolean) => void
  prompt?: VoicePrompt
  renderEditor?: (props: {
    ownerId: string
    voiceId: string
    detail: VoiceSampleDetail
    prompt?: VoicePrompt
    blocked: boolean
    onSaved: () => void
    onCancel: () => void
    onBusyChange: (busy: boolean) => void
  }) => ReactNode
}) {
  const { t } = useTranslation(['voicePreparation', 'voiceMaterialEdit'])
  const [editing, setEditing] = useState(false)
  const [editBusy, setEditBusy] = useState(false)
  const close = () => {
    if (!editBusy) onClose()
  }
  const { detail, isError, refetch } = useVoiceSample(ownerId, voiceId, sampleId)
  return (
    <Sheet
      open
      label={title}
      onClose={close}
      header={
        <Button variant="ghost" disabled={editBusy} onClick={close}>
          {t('closeMaterials')}
        </Button>
      }
    >
      <Typography variant="title" as="h3">
        {title}
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
          onBusyChange: (busy) => {
            setEditBusy(busy)
            onBusyChange?.(busy)
          },
        })
      ) : isError ? (
        <Notice tone="danger" className="mt-4">
          {t('loadFailed')}
          <Button variant="secondary" onClick={refetch}>
            {t('retry')}
          </Button>
        </Notice>
      ) : detail ? (
        <>
          {detail.photoUrl && (
            <img
              src={detail.photoUrl}
              width={detail.photoWidth}
              height={detail.photoHeight}
              alt={t('photo', { ns: 'voiceMaterialEdit' })}
              className="max-h-field mt-4 h-auto w-full rounded-md object-contain"
            />
          )}
          <Typography variant="body" className="mt-4 break-words whitespace-pre-wrap">
            {detail.body}
          </Typography>
          {renderEditor && (
            <Button
              variant="secondary"
              disabled={blocked}
              className="mt-4"
              onClick={() => setEditing(true)}
            >
              {t('edit', { ns: 'voiceMaterialEdit' })}
            </Button>
          )}
        </>
      ) : (
        <Typography variant="body" role="status" className="mt-4">
          {t('preparing')}
        </Typography>
      )}
    </Sheet>
  )
}
