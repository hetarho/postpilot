import { useEffect, useMemo, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { isTerminal, useJob, type GenerationJob } from '@/entities/generation-job'
import { useListPostsQueryKey, usePostQueryKey, type PostDraft } from '@/entities/post'
import type { EditorStep } from './steps'

export interface EditorJobView {
  /** '' while this editor has never started a job and the post carries none. */
  jobId: string
  job: GenerationJob | undefined
  /** True while the id is known but the first poll has not answered yet. */
  isPending: boolean
  isError: boolean
  refetch: () => void
  /** Which step owns the running job — the step whose retry control is mounted. */
  jobStep: EditorStep
  /** The step a job was started FROM, when this editor started it. */
  startedStep: EditorStep | undefined
  onStarted: (jobId: string, step: EditorStep) => void
}

/** The one durable job this editor is watching, resolved above the step panels.
 *
 *  It sits at the page's top level because the page-top status region reports it (`EditorStatus`)
 *  and the step panels act on it, and both need the SAME poll — a second `useJob` for the status
 *  bar would double the 2s polling ([I5]) and let the two surfaces disagree by one interval. */
export function useEditorJob(post: PostDraft | undefined): EditorJobView {
  const queryClient = useQueryClient()
  // The step is recorded with the job because the retry lives there: the generate action is
  // mounted only on 글 생성 and the revise form only on 글 다듬기, so reporting a failure on the
  // other step would offer a retry that reaches nothing.
  const [started, setStarted] = useState<{ id: string; step: EditorStep } | null>(null)
  const slug = post?.slug ?? ''
  const jobId = started?.id || post?.activeJob?.id || ''
  const postKey = usePostQueryKey(slug)
  const postsKey = useListPostsQueryKey()
  const invalidateOnDone = useMemo(
    () => (slug ? [postKey, postsKey] : []),
    [postKey, postsKey, slug],
  )
  const jobState = useJob(jobId, invalidateOnDone)
  const job = jobState.job ?? (post?.activeJob?.id === jobId ? post.activeJob : undefined)

  // Observations are persisted batch-by-batch on the post, not on the job. Refresh that
  // read model whenever observe progress changes so the contact sheet fills while the
  // durable job is still running.
  const refreshedSnapshot = useRef('')
  useEffect(() => {
    if (!job || isTerminal(job) || !slug) return
    const snapshot = `${job.id}:${job.updatedAt}:${job.progressDone}:${job.progressTotal}`
    if (refreshedSnapshot.current === snapshot) return
    refreshedSnapshot.current = snapshot
    void queryClient.invalidateQueries({ queryKey: postKey })
  }, [job, postKey, queryClient, slug])

  return {
    jobId,
    job,
    isPending: Boolean(jobId) && !job,
    isError: jobState.isError,
    refetch: jobState.refetch,
    // A resumed job has no recorded step, so its kind says which one owns it.
    jobStep:
      started?.id === jobId ? started.step : resumedJobStep(job?.kind, Boolean(post?.storyline)),
    startedStep: started?.id === jobId ? started.step : undefined,
    onStarted: (id, step) => setStarted({ id, step }),
  }
}

/** The step a job this editor did not start belongs to. A revision and a storyline request are
 *  글 다듬기's; a storyline job is 글 생성's while the post has no storyline yet (스토리라인 먼저 is
 *  the only way to start one there) and 글 다듬기's once it has one (다시 만들기). */
export function resumedJobStep(kind: string | undefined, hasStoryline: boolean): EditorStep {
  if (kind === 'revise' || kind === 'revise_storyline') return 'refine'
  if (kind === 'storyline') return hasStoryline ? 'refine' : 'generate'
  return 'generate'
}
