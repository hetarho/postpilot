import { FailureNotice, ProgressLine } from '@/entities/generation-job'
import { Button, FieldMessage, Notice } from '@/shared/ui'
import type { VoiceLearning } from '../model/useVoiceLearning'

/** 글 완성's own action, and the one place every learning outcome is reported.
 *
 *  The button is offered for a finalized revision that has not taught the voice anything yet; a
 *  revision already learned from keeps the button in place but disabled, so the completed run is
 *  visible as a state rather than as a control that has vanished. A run started by
 *  확정하고 말투 학습 on the previous step lands here too — the state is held above both panels. */
export function VoiceLearningPanel({
  learning,
  onBackToRefine,
}: {
  learning: VoiceLearning
  onBackToRefine: () => void
}) {
  // Said on screen rather than discovered through a refusal: a post edited after its finalize is
  // back in review and has to be confirmed again before it can teach anything.
  const finalized = learning.revisionFinalized

  return (
    <section aria-labelledby="learning-heading" className="mt-8">
      <h2 id="learning-heading" className="text-lg font-semibold tracking-tight">
        말투 학습
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        확정과 말투 학습은 별개예요. 확정만 해도 글은 완료되며, 학습은 버튼을 눌렀을 때만
        시작합니다.
      </p>
      {learning.isError ? (
        <div className="mt-3">
          <FailureNotice error="학습 상태를 확인하지 못했어요." onRetry={learning.refetch} />
        </div>
      ) : learning.retryable && learning.job ? (
        <div className="mt-3">
          <FailureNotice error={learning.job.error} onRetry={() => void learning.retry()} />
        </div>
      ) : learning.active && learning.job ? (
        <div className="mt-3">
          <ProgressLine job={learning.job} />
        </div>
      ) : learning.learned ? (
        <Notice tone="success" role="status" className="mt-3">
          이 글에서 말투를 배웠어요.
        </Notice>
      ) : finalized ? (
        <Notice tone="success" role="status" className="mt-3">
          이 revision을 확정했어요.
        </Notice>
      ) : null}
      {learning.errorMessage && (
        <FieldMessage className="mt-2">
          글은 확정됐지만 말투 학습은 시작하지 못했어요. {learning.errorMessage}
        </FieldMessage>
      )}
      {/* The voice gate outranks the finalize gate: telling someone to confirm a revision that
          could never be learned from either way would be a detour to the same refusal. */}
      {learning.blocked ? (
        <p role="status" className="text-content-secondary mt-2 text-sm">
          {learning.blocked}
        </p>
      ) : !finalized ? (
        <p role="status" className="text-content-secondary mt-2 text-sm">
          아직 확정하지 않은 내용이에요. 글 다듬기에서 확정하면 이 글로 말투를 배울 수 있어요.
        </p>
      ) : (
        learning.needsAnalyzeModel && (
          <p className="text-content-tertiary mt-2 text-sm">
            말투 학습을 하려면 분석 모델을 선택해 주세요.
          </p>
        )
      )}
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          variant="cta"
          disabled={!learning.readyToLearn}
          pending={learning.pending}
          onClick={() => void learning.learn().catch(() => undefined)}
        >
          말투 학습
        </Button>
        {!finalized && !learning.blocked && (
          <Button variant="secondary" onClick={onBackToRefine}>
            글 다듬기로 가기
          </Button>
        )}
        {learning.noTextEdit && learning.learned && !learning.satisfied && (
          <Button
            variant="secondary"
            pending={learning.feedbackPending}
            onClick={() => void learning.satisfy()}
          >
            수정 없이도 마음에 들어요
          </Button>
        )}
      </div>
    </section>
  )
}
