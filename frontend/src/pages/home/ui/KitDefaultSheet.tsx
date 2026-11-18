import { useTranslation } from 'react-i18next'
import { VoiceRow, useVoices } from '@/entities/voice'
import { StageModelRow } from '@/features/configure-model-pair'
import { PostCreditEstimate } from '@/features/select-model'
import { SetDefaultVoiceButton } from '@/features/set-default-voice'
import { GuidelineDirectory } from '@/widgets/guideline-directory'
import { KitPieceSheet, type KitDefaultKey } from '@/widgets/writing-kit'
import { Button, LoadingState, Notice, Typography } from '@/shared/ui'

/** 홈's sheet for one piece's account default, holding the very control the piece's own screen
 *  changes it with — nothing re-made for 홈:
 *  - 말투: every voice as the voices screen lists it, each made one with its 기본으로 설정 /
 *    기본 해제 (VOICE-12); one still being made says how far it is and offers neither.
 *  - 지침: the post-guideline list as the editor opens it in a sheet (POST.md's 지침 바꾸기), so a
 *    기본 지침 goes into use through its `기본 지침` sheet and out through its row's `적용 안함`,
 *    the only two surfaces that change one (GUIDE-43, GUIDE-48).
 *  - 글 쓰는 AI: AI 토너먼트's 지금 쓰는 AI row for the write stage — its grade, its credits per post
 *    and a saved model that can no longer be used — with its picker under it, then the per-post
 *    estimate the tab shows under the rows. */
export function KitDefaultSheet({
  ownerId,
  kit,
  open,
  onClose,
}: {
  ownerId: string
  kit: KitDefaultKey
  open: boolean
  onClose: () => void
}) {
  return (
    <KitPieceSheet
      kit={kit}
      open={open}
      onClose={onClose}
      size={kit === 'guideline' ? 'wide' : 'default'}
    >
      {kit === 'voice' && <VoiceDefault ownerId={ownerId} />}
      {kit === 'guideline' && <GuidelineDirectory kind="post" embedded />}
      {kit === 'ai' && (
        <>
          <StageModelRow stage="write" inline />
          <PostCreditEstimate className="mt-4" />
        </>
      )}
    </KitPieceSheet>
  )
}

function VoiceDefault({ ownerId }: { ownerId: string }) {
  const { t } = useTranslation(['creation', 'voices', 'common'])
  const { active, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  if (isError)
    return (
      <Notice tone="danger" role="alert">
        <span>{t('voiceLoadFailed', { ns: 'voices' })}</span>
        <Button
          variant="ghost"
          onClick={refetch}
          pending={isFetching}
          className="text-notice-danger-fg underline"
        >
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Notice>
    )
  if (isPending) return <LoadingState variant="list" label={t('state.loading', { ns: 'common' })} />
  if (active.length === 0)
    return (
      <Typography variant="body" as="p" role="status" className="text-content-secondary">
        {t('kit.sheet.voicesEmpty')}
      </Typography>
    )
  return (
    <ul aria-label={t('kit.sheet.voices')} className="divide-divider divide-y">
      {active.map((voice) => (
        <VoiceRow
          key={voice.id}
          ownerId={ownerId}
          voice={voice}
          action={<SetDefaultVoiceButton ownerId={ownerId} voice={voice} />}
        />
      ))}
    </ul>
  )
}
