import { useState } from 'react'
import type { PostDraft } from '@/entities/post'
import { Button, Dialog, FieldMessage, Notice } from '@/shared/ui'
import { useFinalizePost } from '../api/useFinalizePost'
import type { VoiceLearning } from '../model/useVoiceLearning'

type FinalizeMode = 'finalize' | 'learn'

/** The bottom of 글 다듬기: the boundary that ends the drafting loop.
 *
 *  It sits at the end of the step whose work it closes — you finish reading the draft and the
 *  next thing under your thumb is the way out of it — and both actions carry the user to 글 완성,
 *  so finishing a step is one gesture rather than a click plus a tab change. 확정 records the
 *  revision and nothing else; 확정하고 말투 학습 additionally starts the learning run, which is
 *  reported on 글 완성 because that is where it lands. */
export function FinalizeActions({
  post,
  learning,
  beforeFinalize,
  onFinalized,
}: {
  post: PostDraft
  learning: VoiceLearning
  /** Flushes pending edits and resolves the exact revision to finalize. */
  beforeFinalize: () => Promise<bigint>
  onFinalized: () => void
}) {
  const finalize = useFinalizePost()
  const [confirming, setConfirming] = useState<FinalizeMode | ''>('')
  const [preparing, setPreparing] = useState<FinalizeMode | ''>('')
  // The status is the state (policy/posts.md): a content save after a finalize returns the post
  // to `review`, so this comes back on its own when the user edits again.
  const finalized = post.status === 'finalized'

  const run = async (mode: FinalizeMode) => {
    if (mode === 'learn' && !learning.canLearn) return
    setPreparing(mode)
    try {
      const revision = await beforeFinalize()
      await finalize.finalize(post.slug, revision)
      setConfirming('')
      // A learning failure does not undo the finalize written immediately before it, and it is
      // reported on 글 완성 — so the step moves either way.
      if (mode === 'learn') await learning.learn(revision).catch(() => undefined)
      onFinalized()
    } catch {
      // The finalize itself failed. Its mutation renders the error below, and the step holds.
    } finally {
      setPreparing('')
    }
  }

  return (
    <section aria-labelledby="finalize-heading" className="mt-12">
      <h2 id="finalize-heading" className="text-lg font-semibold tracking-tight">
        확정
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        {finalized
          ? '이 내용은 이미 확정했어요. 글 완성에서 내보내거나 발행할 수 있습니다.'
          : '다 다듬었다면 지금 내용을 확정해 주세요. 확정하면 글 완성으로 넘어갑니다.'}
      </p>
      {finalize.isError && <FieldMessage className="mt-2">글을 확정하지 못했어요.</FieldMessage>}
      {finalized ? (
        <>
          <Notice tone="success" role="status" className="mt-3">
            이 revision을 확정했어요.
          </Notice>
          <Button variant="secondary" className="mt-4" onClick={onFinalized}>
            글 완성으로 가기
          </Button>
        </>
      ) : (
        <>
          {learning.blocked ? (
            <p role="status" className="text-content-secondary mt-2 text-sm">
              {learning.blocked}
            </p>
          ) : (
            learning.needsAnalyzeModel && (
              <p className="text-content-tertiary mt-2 text-sm">
                말투 학습을 하려면 분석 모델을 선택해 주세요. 확정만 하는 데에는 필요하지 않아요.
              </p>
            )
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={!post.canFinalize || learning.active}
              pending={preparing === 'finalize'}
              onClick={() => setConfirming('finalize')}
            >
              확정
            </Button>
            <Button
              variant="cta"
              disabled={!post.canFinalize || !learning.canLearn}
              pending={preparing === 'learn'}
              onClick={() => setConfirming('learn')}
            >
              확정하고 말투 학습
            </Button>
          </div>
        </>
      )}
      <Dialog
        open={Boolean(confirming)}
        title="이 revision을 확정할까요?"
        confirmLabel={confirming === 'learn' ? '확정하고 학습' : '확정'}
        pending={Boolean(preparing)}
        onClose={() => setConfirming('')}
        onConfirm={() => confirming && void run(confirming)}
      >
        현재 편집 내용을 먼저 저장한 뒤 정확한 revision을 확정하고 글 완성으로 넘어갑니다.
        {confirming === 'learn'
          ? ' 그 다음에만 말투 학습을 시작합니다.'
          : ' 모델 호출이나 말투 학습은 하지 않습니다.'}
      </Dialog>
    </section>
  )
}
