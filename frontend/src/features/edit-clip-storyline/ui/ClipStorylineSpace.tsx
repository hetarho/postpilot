import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useSaveClipStoryline,
  type ClipProject,
  type ClipStorylineParagraph,
} from '@/entities/clip-project'
import { ActionMenu, AppFailureMessage, Disclosure, Notice, Typography } from '@/shared/ui'
import { clipScenes, takenOutScenes, withSceneIn } from '../model/storyline-edits'
import { queueClipStoryline } from '../model/storyline-queue'
import { useClipStorylineQueue } from '../model/useClipStorylineQueue'
import { ClipSceneFrame } from './ClipSceneFrame'
import { ClipStorylineParagraphEditor } from './ClipStorylineParagraphEditor'

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
  const projectId = project.id
  const { owed, failure } = useClipStorylineQueue(projectId)
  // What a previous mount still owed the server outranks the server's: it is newer by exactly the
  // words typed since.
  const [draft, setDraft] = useState<ClipStorylineParagraph[]>(
    () => owed ?? storyline?.paragraphs ?? [],
  )
  const server = JSON.stringify(storyline?.paragraphs ?? [])
  // The server's storyline replaces the draft only while nothing of the owner's is queued, in
  // flight or failed for this project: a storyline job, a request or another tab wrote a new one.
  // A failed save keeps the typed text. Adjusted during render, not from an effect, so a frame of
  // the older words is never painted.
  const [adopted, setAdopted] = useState(server)
  if (adopted !== server && !owed) {
    setAdopted(server)
    setDraft(JSON.parse(server) as ClipStorylineParagraph[])
  }
  const change = (next: ClipStorylineParagraph[]) => {
    setDraft(next)
    queueClipStoryline(projectId, next, async (paragraphs) => {
      await save.mutateAsync({ projectId, paragraphs })
    })
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
      {failure && (
        <div role="alert" className="mt-3">
          <AppFailureMessage failure={failure} />
        </div>
      )}
    </Disclosure>
  )
}
