import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useVoicePrompts, useVoiceSample, type VoiceSample } from '@/entities/voice'
import { Button, Notice, Sheet, Typography } from '@/shared/ui'
export function LearningMaterials({
  ownerId,
  voiceId,
  samples,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
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
          ownerId={ownerId}
          voiceId={voiceId}
          sampleId={opened.id}
          title={title(opened)}
          onClose={() => setOpened(null)}
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
}: {
  ownerId: string
  voiceId: string
  sampleId: string
  title: string
  onClose: () => void
}) {
  const { t } = useTranslation('voicePreparation')
  const { detail, isError, refetch } = useVoiceSample(ownerId, voiceId, sampleId)
  return (
    <Sheet
      open
      label={title}
      onClose={onClose}
      header={
        <Button variant="ghost" onClick={onClose}>
          {t('closeMaterials')}
        </Button>
      }
    >
      <Typography variant="title" as="h3">
        {title}
      </Typography>
      {isError ? (
        <Notice tone="danger" className="mt-4">
          {t('loadFailed')}
          <Button variant="secondary" onClick={refetch}>
            {t('retry')}
          </Button>
        </Notice>
      ) : detail ? (
        <Typography variant="body" className="mt-4 break-words whitespace-pre-wrap">
          {detail.body}
        </Typography>
      ) : (
        <Typography variant="body" role="status" className="mt-4">
          {t('preparing')}
        </Typography>
      )}
    </Sheet>
  )
}
