import { create } from '@bufbuild/protobuf'
import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useMutation } from '@connectrpc/connect-query'
import { useStageSelection } from '@/entities/model-catalog'
import type { VoiceProfile } from '@/entities/voice'
import { ModelRefSchema, VoiceValidationService } from '@/shared/api'
import { VOICE_VALIDATION_POST_COUNT } from '@/shared/config'
import { Button, Checkbox, Dialog, Notice } from '@/shared/ui'

/** Starts a validation of ONE voice's profile against that voice's own finalized sources; the
 *  request names the voice explicitly (spec/policy/voice.md). */
export function ValidateVoiceProfile({
  voiceId,
  profile,
  blocked = '',
}: {
  voiceId: string
  profile: VoiceProfile
  blocked?: string
}) {
  const analyze = useStageSelection('analyze')
  const write = useStageSelection('write')
  const mutation = useMutation(VoiceValidationService.method.startVoiceProfileValidation)
  const navigate = useNavigate()
  const [judge, setJudge] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const missing = Math.max(0, VOICE_VALIDATION_POST_COUNT - profile.finalizedSourceCount)
  const start = async () => {
    if (blocked || !analyze.selected || !write.selected) return
    const response = await mutation.mutateAsync({
      voiceId,
      analyzeModel: create(ModelRefSchema, analyze.selected),
      writeModel: create(ModelRefSchema, write.selected),
      judgeEnabled: judge,
    })
    setConfirming(false)
    if (response.validationId) {
      void navigate({
        to: '/voices/$voiceId/validations/$id',
        params: { voiceId, id: response.validationId },
      })
    }
  }
  return (
    <section aria-label="프로필 검증">
      <label className="text-content-secondary flex min-h-11 items-center gap-3 text-sm">
        <Checkbox
          checked={judge}
          disabled={Boolean(blocked)}
          onChange={(event) => setJudge(event.target.checked)}
        />
        AI 심사로 5개 항목의 일치율도 계산
      </label>
      <Button
        variant="secondary"
        className="mt-3"
        disabled={Boolean(blocked) || !profile.canValidate || !analyze.selected || !write.selected}
        onClick={() => setConfirming(true)}
      >
        검증 시작
      </Button>
      {missing > 0 && (
        <p className="text-content-tertiary mt-2 text-sm">
          완성하고 학습한 글이 {missing}편 더 필요해요.
        </p>
      )}
      {blocked && (
        <Notice tone="warning" role="status" className="mt-3">
          {blocked}
        </Notice>
      )}
      <Dialog
        open={confirming}
        title="프로필 검증을 시작할까요?"
        confirmLabel="검증 작업 시작"
        pending={mutation.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() => void start()}
      >
        분석 모델과 작성 모델을 명시적으로 사용합니다.
        {judge ? ' AI 심사 호출도 포함합니다.' : ' AI 심사는 호출하지 않습니다.'}
      </Dialog>
    </section>
  )
}
