import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useSaveClipStoryline,
  type ClipProject,
  type ClipStorylineParagraph,
} from '@/entities/clip-project'
import { appFailureFromConnect } from '@/shared/api'
import { ActionMenu, AppFailureMessage, Disclosure, Notice, Typography } from '@/shared/ui'
import { clipScenes, takenOutScenes, withSceneIn } from '../model/storyline-edits'
import { ClipSceneFrame } from './ClipSceneFrame'
import { ClipStorylineParagraphEditor } from './ClipStorylineParagraphEditor'

// A beat after the last edit, so one save carries a word, not every keystroke.
const SAVE_DELAY_MS = 600

/** ②'s storyline space (CLIP-178, CLIP-179): the intro, the clip's storyline as paragraphs with
 *  their scenes as frames, edited and rearranged by hand, and the outro. Open while the project has
 *  no plan, and closed when ② first shows one — the plan is then what the owner works on. Every
 *  edit saves itself through the project update with no dirty gate (CLIP-39); a running job or a
 *  finalized project leaves it read-only (CLIP-160). `aside` and `lead` are the heading row's
 *  actions and the field under it, which the storyline actions fill; `intro` and `outro` are the
 *  region blocks, which stand here before any storyline exists (CLIP-179). */
export function ClipStorylineSpace({
  ownerId,
  project,
  hasPlan,
  readOnly,
  localSources,
  resolvePlayback,
  aside,
  lead,
  intro,
  outro,
}: {
  ownerId: string
  project: ClipProject
  hasPlan: boolean
  readOnly: boolean
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  aside?: ReactNode
  lead?: ReactNode
  intro?: ReactNode
  outro?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const storyline = project.storyline
  const [open, setOpen] = useState(!hasPlan)
  const hadPlan = useRef(hasPlan)
  useEffect(() => {
    if (hasPlan && !hadPlan.current) setOpen(false)
    hadPlan.current = hasPlan
  }, [hasPlan])

  const save = useSaveClipStoryline(ownerId)
  const [draft, setDraft] = useState<ClipStorylineParagraph[]>(storyline?.paragraphs ?? [])
  const timer = useRef<number | undefined>(undefined)
  const pending = useRef<ClipStorylineParagraph[] | undefined>(undefined)
  const server = JSON.stringify(storyline?.paragraphs ?? [])
  // The server's storyline replaces the draft whenever nothing of the owner's is waiting to be
  // saved: a storyline job, a request or another tab wrote a new one.
  useEffect(() => {
    if (!pending.current && !save.isPending)
      setDraft(JSON.parse(server) as ClipStorylineParagraph[])
  }, [server, save.isPending])
  const flush = () => {
    window.clearTimeout(timer.current)
    timer.current = undefined
    const next = pending.current
    if (!next) return
    pending.current = undefined
    save.mutate({ projectId: project.id, paragraphs: next })
  }
  // The latest flush, for the timer and the unmount — kept current after each render, never
  // during one.
  const flushRef = useRef(flush)
  useEffect(() => {
    flushRef.current = flush
  })
  useEffect(() => () => flushRef.current(), [])
  const change = (next: ClipStorylineParagraph[]) => {
    setDraft(next)
    pending.current = next
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => flushRef.current(), SAVE_DELAY_MS)
  }

  const scenes = useMemo(() => clipScenes(project.observations), [project.observations])
  const takenOut = storyline ? takenOutScenes(storyline, draft) : []
  return (
    <Disclosure
      title={t('storylineSpace.title')}
      open={open}
      onOpenChange={setOpen}
      aside={aside}
      lead={lead}
      className="mb-6"
    >
      {!!storyline?.addedSourceIds.length && (
        <Notice tone="info" role="status" className="mt-2">
          {t('storylineSpace.added')}
        </Notice>
      )}
      {intro}
      {draft.length > 0 && (
        <ol className="divide-divider divide-y">
          {draft.map((_, index) => (
            <ClipStorylineParagraphEditor
              key={index}
              paragraphs={draft}
              index={index}
              scenes={scenes}
              readOnly={readOnly}
              localSources={localSources}
              resolvePlayback={resolvePlayback}
              onChange={change}
            />
          ))}
        </ol>
      )}
      {takenOut.length > 0 && (
        <section aria-labelledby="clip-storyline-taken-out" className="mt-4">
          <Typography variant="label" as="h3" id="clip-storyline-taken-out">
            {t('storylineSpace.takenOut')}
          </Typography>
          <ul className="mt-2 flex flex-wrap gap-3">
            {takenOut.map((id) => {
              const scene = scenes.get(id)
              const name = scene ? t('storylineSpace.scene', { n: scene.number }) : id
              return (
                <li key={id} className="flex flex-col items-start gap-1">
                  <ClipSceneFrame
                    scene={scene}
                    sceneId={id}
                    localSources={localSources}
                    resolvePlayback={resolvePlayback}
                  />
                  {!readOnly && (
                    <ActionMenu
                      label={t('storylineSpace.putBack', { scene: name })}
                      triggerLabel={t('storylineSpace.putBackLabel')}
                      items={draft.map((_, at) => ({
                        id: `p${at}`,
                        label: t('storylineSpace.paragraph', { n: at + 1 }),
                        onSelect: () => change(withSceneIn(draft, id, at)),
                      }))}
                    />
                  )}
                </li>
              )
            })}
          </ul>
        </section>
      )}
      {outro}
      {save.isError && (
        <div role="alert" className="mt-3">
          <AppFailureMessage failure={appFailureFromConnect(save.error)} />
        </div>
      )}
    </Disclosure>
  )
}
