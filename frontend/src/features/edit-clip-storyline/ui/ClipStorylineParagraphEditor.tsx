import { useTranslation } from 'react-i18next'
import { ArrowRightLeft } from 'lucide-react'
import type { ClipStorylineParagraph } from '@/entities/clip-project'
import { Button, Editable, Menu, Textarea, Typography } from '@/shared/ui'
import { withSceneIn, withText, withoutScene, type ClipScene } from '../model/storyline-edits'
import { ClipSceneFrame } from './ClipSceneFrame'

/** One paragraph of the clip storyline, edited by hand (CLIP-178): its number, its text edited in
 *  place, and its scenes as frames, each with a move control — the paragraph to move it to, or
 *  빼기. Every change hands the whole list back, which is what the space saves. */
export function ClipStorylineParagraphEditor({
  paragraphs,
  index,
  scenes,
  readOnly,
  localSources,
  resolvePlayback,
  onChange,
}: {
  paragraphs: readonly ClipStorylineParagraph[]
  index: number
  scenes: ReadonlyMap<string, ClipScene>
  readOnly: boolean
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  onChange: (paragraphs: ClipStorylineParagraph[]) => void
}) {
  const { t } = useTranslation('clips')
  const paragraph = paragraphs[index]!
  const number = index + 1
  const moveOptions = [
    ...paragraphs.map((_, at) => ({
      value: `p${at}`,
      label: t('storylineSpace.paragraph', { n: at + 1 }),
    })),
    { value: 'out', label: t('storylineSpace.takeOut') },
  ]
  return (
    <li className="py-4" aria-label={t('storylineSpace.paragraph', { n: number })}>
      <Typography variant="label" as="p" className="text-content-secondary">
        {t('storylineSpace.paragraph', { n: number })}
      </Typography>
      <Editable
        className="mt-1"
        readOnly={readOnly}
        editLabel={t('storylineSpace.editText', { n: number })}
        edit={(exit) => (
          <div className="flex flex-col gap-2">
            <Textarea
              autoGrow
              autoFocus
              rows={2}
              aria-label={t('storylineSpace.paragraph', { n: number })}
              value={paragraph.text}
              onChange={(event) => onChange(withText(paragraphs, index, event.currentTarget.value))}
            />
            <Button variant="secondary" className="self-end" onClick={exit}>
              {t('storylineSpace.done')}
            </Button>
          </div>
        )}
      >
        <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
          {paragraph.text}
        </Typography>
      </Editable>
      {paragraph.observationIds.length > 0 && (
        <ul className="mt-3 flex flex-wrap gap-3">
          {paragraph.observationIds.map((id) => {
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
                  <Menu
                    label={t('storylineSpace.move', { scene: name })}
                    value={`p${index}`}
                    options={moveOptions}
                    triggerIcon={<ArrowRightLeft aria-hidden="true" className="size-4" />}
                    onChange={(value) =>
                      onChange(
                        value === 'out'
                          ? withoutScene(paragraphs, id)
                          : withSceneIn(paragraphs, id, Number(value.slice(1))),
                      )
                    }
                  />
                )}
              </li>
            )
          })}
        </ul>
      )}
    </li>
  )
}
