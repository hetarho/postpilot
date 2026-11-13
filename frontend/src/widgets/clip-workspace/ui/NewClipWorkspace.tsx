import { useEffect, useId, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { DEFAULT_CLIP_CHANNEL, type ClipChannel } from '@/entities/clip-project'
import {
  ClipProjectForm,
  focusedClipField,
  newClipDraftKey,
  stashClipMintHandoff,
  useClipDraftSave,
  type ClipPreparationScreen,
} from '@/features/edit-clip-project'
import { ClipSourcePicker, useClipSourceUpload } from '@/features/upload-clip-sources'
import { Button } from '@/shared/ui'
import { ClipPreparationGuide } from './ClipPreparationGuide'
import { ClipStatusLine } from './ClipStatus'
import { ClipTopRow } from './ClipTopRow'

/** `/clips/new`: an unminted project running ①'s guide (CLIP-130, CLIP-225). It has no lifecycle
 *  yet, so it shows no step bar and no delete (CLIP-36), and no action creates it: its first saved
 *  change — a choice, an answer, typed text or a picked source — mints the 9:16 project in the
 *  channel the page was opened from, and the address becomes `/clips/<id>` with the guide screen
 *  in use and the caret handed to the workspace that address mounts (CLIP-37, POST-11). */
export function NewClipWorkspace({
  ownerId,
  channel = DEFAULT_CLIP_CHANNEL,
  initialTemplateId,
  initialGuide,
  returnTo,
}: {
  ownerId: string
  channel?: ClipChannel
  /** The video template the new-clip offer's wizard just saved (CLIP-223). */
  initialTemplateId?: string
  /** Back from the template wizard: 템플릿 (CLIP-225). */
  initialGuide?: ClipPreparationScreen
  returnTo: { href: string; label: string; onReturn?: () => void | Promise<void> }
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const [draftKey] = useState(newClipDraftKey)
  const save = useClipDraftSave(draftKey)
  const [guide, setGuide] = useState<ClipPreparationScreen>(initialGuide ?? 'kind')
  // The mint resolves after the owner may have moved on: it hands over the screen in use NOW.
  const guideNow = useRef(guide)
  useEffect(() => {
    guideNow.current = guide
  }, [guide])
  const root = useRef<HTMLDivElement>(null)
  const reasonId = useId()
  // No session runs here: a pick mints first and uploads to the minted id (CLIP-130).
  const upload = useClipSourceUpload('', undefined, false)
  const minted = useRef(false)
  // A mint answering after the owner left this page navigates nowhere: the address it would
  // replace is no longer the one on screen. The project keeps its frozen creation origin.
  const alive = useRef(false)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  const onMinted = (id: string) => {
    if (minted.current || !alive.current) return
    minted.current = true
    stashClipMintHandoff({
      clipId: id,
      screen: guideNow.current,
      caret: focusedClipField(root.current),
    })
    // Replace: Back returns to where the creation started, never to an unminted page (POST-11).
    void navigate({
      to: '/clips/$clipId',
      params: { clipId: id },
      search: { channel },
      replace: true,
    })
  }
  return (
    <div ref={root} className="flex flex-1 flex-col">
      {/* The status region is mounted before there is a project to have a status: a live region
          inserted already holding its message announces nothing. */}
      <ClipTopRow
        returnTo={returnTo}
        status={
          <ClipStatusLine
            project={undefined}
            job={undefined}
            upload={{ phase: 'idle' }}
            correction="clean"
            save={save}
            className="order-last w-full"
          />
        }
        actions={
          save.failing && (
            // A refused or failed mint keeps every field; the owner sends it again from here.
            <Button variant="ghost" onClick={() => void save.flush(true).catch(() => undefined)}>
              {t('project.retry')}
            </Button>
          )
        }
      />
      <ClipProjectForm
        ownerId={ownerId}
        channel={channel}
        initialTemplateId={initialTemplateId}
        unmintedKey={draftKey}
        onMinted={onMinted}
        layout={(content) => (
          <ClipPreparationGuide
            screen={guide}
            onSelect={setGuide}
            reasonId={reasonId}
            campaign={content.campaign}
            requirements={content.requirements}
            noteEmpty={content.noteEmpty}
            screens={{
              ...content.screens,
              sources: (
                <>
                  {content.screens.sources}
                  <ClipSourcePicker upload={upload} ensureProject={content.mint} />
                </>
              ),
            }}
            rows={content.rows}
            models={null}
            refusal={content.refusal}
            // Nothing is entered yet: the way to make it possible is to pick footage (POST-123).
            correction={{ screen: 'sources', label: t('guide.noSources') }}
          />
        )}
      />
    </div>
  )
}
