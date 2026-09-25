import clsx from 'clsx'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type ClipCaptionFragment,
  type ClipRegionPresetSample,
  useClipCaptionStyleSamples,
  useClipRegionPresetSamples,
} from '@/entities/clip-plan'
import { type ClipProjectDraft } from '@/entities/clip-project'
import {
  CLIP_CAPTION_STYLES,
  CLIP_DEFAULT_CAPTION_STYLE,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
} from '@/entities/clip-design'
import { Checkbox, Typography } from '@/shared/ui'

/** One style as the RENDERER draws it, scaled into the row (CDS-83). The
 *  fragment is already at the origin, so its own box is the whole viewBox. */
function StyleSample({ fragment }: { fragment?: ClipCaptionFragment }) {
  if (!fragment || fragment.box.width <= 0 || fragment.box.height <= 0) return null
  return (
    <svg
      viewBox={`0 0 ${fragment.box.width} ${fragment.box.height}`}
      className="h-8 w-full"
      preserveAspectRatio="xMinYMid meet"
      aria-hidden="true"
      /* The server's own drawing of this style: there is no other way to show
         the very SVG the renderer produces (CDS-83). */
      dangerouslySetInnerHTML={{ __html: fragment.svg }}
    />
  )
}

/** One region's presets as a radiogroup of tiles (CLIP-111, CLIP-165): each tile is
 *  the renderer's drawing of the preset's numbered slots on the whole canvas, so an
 *  edge-anchored preset shows at its true place, above the preset's name. Only the
 *  chosen tile is tabbable and the arrows move focus and choice together (WAI-APG
 *  radio group). While the drawings load, or when they fail, the tiles show names
 *  and stay selectable. */
function PresetPicker<T extends string>({
  kind,
  ids,
  value,
  samples,
  canvas,
  name,
  onChange,
}: {
  kind: 'intro' | 'outro'
  ids: readonly T[]
  value: T
  samples?: ClipRegionPresetSample[]
  canvas: { width: number; height: number }
  name: (id: T) => string
  onChange: (next: T) => void
}) {
  const { t } = useTranslation('clips')
  const label = useId()
  const drawing = new Map((samples ?? []).map((s) => [s.preset, s.svg]))
  return (
    <div
      role="radiogroup"
      aria-labelledby={label}
      className="min-w-0"
      onKeyDown={(event) => {
        const step =
          event.key === 'ArrowDown' || event.key === 'ArrowRight'
            ? 1
            : event.key === 'ArrowUp' || event.key === 'ArrowLeft'
              ? -1
              : 0
        if (step === 0) return
        event.preventDefault()
        const next = (ids.indexOf(value) + step + ids.length) % ids.length
        onChange(ids[next])
        event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="radio"]')[next]?.focus()
      }}
    >
      <Typography id={label} variant="fieldTitle" as="p">
        {t(`composition.design.${kind}`)}
      </Typography>
      <div className="mt-2 grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-4">
        {ids.map((id) => {
          const svg = drawing.get(id)
          const chosen = id === value
          return (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={chosen}
              tabIndex={chosen ? 0 : -1}
              onClick={() => onChange(id)}
              className={clsx(
                'flex min-w-0 flex-col gap-2 rounded-md p-2 text-left transition-colors',
                chosen
                  ? 'bg-row-bg-active text-content-primary ring-stroke-accent ring-2'
                  : 'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active',
              )}
            >
              <span
                aria-hidden="true"
                className="bg-media-canvas-bg block w-full overflow-hidden rounded-sm"
                style={{ aspectRatio: `${canvas.width} / ${canvas.height}` }}
              >
                {svg && (
                  <svg
                    viewBox={`0 0 ${canvas.width} ${canvas.height}`}
                    className="block h-full w-full"
                    /* The renderer's own drawing of this preset's slots: ① shows
                       it rather than a picture of it (CLIP-159, CLIP-165). */
                    dangerouslySetInnerHTML={{ __html: svg }}
                  />
                )}
              </span>
              <Typography variant="body" as="span" className="min-w-0 break-words">
                {name(id)}
              </Typography>
            </button>
          )
        })}
      </div>
    </div>
  )
}

const INTRO_IDS = Object.keys(
  CLIP_DESIGN.regions.intro,
) as (keyof typeof CLIP_DESIGN.regions.intro)[]
const OUTRO_IDS = Object.keys(
  CLIP_DESIGN.regions.outro,
) as (keyof typeof CLIP_DESIGN.regions.outro)[]

/** The rest of ①'s design selection (CLIP-111, CLIP-139, CLIP-142): the preset
 *  the intro is drawn in, the preset the outro is drawn in and the caption
 *  styles this clip may use. Each is the project's own and each change re-renders
 *  the same plan without a writing call, so nothing here asks for approval. */
export function ClipDesignSelection({
  projectId,
  draft,
  onChange,
}: {
  projectId: string
  draft: ClipProjectDraft
  onChange: (next: Partial<ClipProjectDraft>) => void
}) {
  const { t } = useTranslation('clips')
  const samples = useClipCaptionStyleSamples(projectId, true)
  const regions = useClipRegionPresetSamples(projectId, t('composition.design.slotLabel'))
  const canvas = CLIP_DESIGN.ratios[draft.ratio].canvas
  const byStyle = new Map((samples.data?.captions ?? []).map((c) => [c.style, c]))
  const selected = draft.allowedCaptionStyles ?? []
  const toggle = (style: string, on: boolean) =>
    onChange({
      allowedCaptionStyles: on
        ? CLIP_CAPTION_STYLES.filter((id) => id === style || selected.includes(id))
        : selected.filter((id) => id !== style),
    })
  return (
    <div className="space-y-3">
      <PresetPicker
        kind="intro"
        ids={INTRO_IDS}
        value={draft.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro}
        samples={regions.data?.intro}
        canvas={canvas}
        name={(id) => t(`composition.design.intro_${id}`)}
        onChange={(introPreset) => onChange({ introPreset })}
      />
      <PresetPicker
        kind="outro"
        ids={OUTRO_IDS}
        value={draft.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro}
        samples={regions.data?.outro}
        canvas={canvas}
        name={(id) => t(`composition.design.outro_${id}`)}
        onChange={(outroPreset) => onChange({ outroPreset })}
      />
      <Typography variant="fieldTitle" as="p">
        {t('project.captionStyles')}
      </Typography>
      <div
        role="group"
        aria-label={t('project.captionStyles')}
        className="grid gap-2 sm:grid-cols-2"
      >
        {CLIP_CAPTION_STYLES.map((style) => {
          const fragment = byStyle.get(style)
          return (
            <label key={style} className="flex min-h-11 min-w-0 items-center gap-3">
              <Checkbox
                checked={selected.includes(style)}
                onChange={(event) => toggle(style, event.target.checked)}
              />
              <span className="min-w-0 flex-1">
                <Typography variant="body" as="span" className="block break-words">
                  {t(`captionStyles.${style}`)}
                  {fragment?.representativeFrame && (
                    <Typography variant="meta" as="span" className="text-content-secondary ml-2">
                      {t('project.perFrameStyle')}
                    </Typography>
                  )}
                </Typography>
                <StyleSample fragment={fragment} />
              </span>
            </label>
          )
        })}
      </div>
      <Typography variant="body" className="text-content-secondary">
        {selected.length
          ? t('project.captionStylesHelp')
          : t('project.captionStylesEmpty', {
              style: t(`captionStyles.${CLIP_DEFAULT_CAPTION_STYLE}`),
            })}
      </Typography>
      {samples.isError && (
        <Typography variant="body" className="text-content-secondary">
          {t('project.captionStylesUnavailable')}
        </Typography>
      )}
    </div>
  )
}
