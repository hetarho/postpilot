import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { EMPTY_GUIDE } from '@/entities/advertiser-guide'
import {
  clipChannel,
  useClipUploadTexts,
  useSaveClipChecklistTicks,
  type ClipProject,
} from '@/entities/clip-project'
import { ClipAdvertiserChecklist } from '@/features/check-advertiser-guide'
import { Typography } from '@/shared/ui'
import { clipChecklistMaterial } from '../model/finishChecklist'

/** ③'s pre-submission checklist while 체험단 글 is on (CLIP-231), read against the confirmed plan
 *  and the channel's saved 업로드 문구. A tick saves at once as its own option save, on a
 *  finalized clip too (CLIP-160); it gates nothing. */
export function ClipFinishChecklist({
  ownerId,
  project,
}: {
  ownerId: string
  project: ClipProject
}) {
  const { t } = useTranslation('clips')
  const texts = useClipUploadTexts(ownerId, project.id)
  const save = useSaveClipChecklistTicks(ownerId)
  const [ticks, setTicks] = useState<readonly string[]>(project.checklistTicks ?? [])
  const channel = clipChannel(project.channel)
  const upload = texts.data?.texts.find((text) => text.channel === channel)
  return (
    <>
      <ClipAdvertiserChecklist
        guide={project.advertiserGuide ?? EMPTY_GUIDE}
        coverage={clipChecklistMaterial(project, upload)}
        ticks={ticks}
        readOnly={false}
        onTicksChange={(next) => {
          setTicks(next)
          save.mutate({ id: project.id, ticks: next })
        }}
      />
      {save.isError && (
        <Typography variant="meta" role="alert" className="mt-2">
          {t('finish.ticksFailed')}
        </Typography>
      )}
    </>
  )
}
