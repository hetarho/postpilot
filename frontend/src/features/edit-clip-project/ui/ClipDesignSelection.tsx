import { useClipLocalStyleSamples, useClipLocalPresetSamples } from '@/entities/clip-preview'
import clsx from 'clsx'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipCaptionStyleSample, type ClipRegionPresetSample } from '@/entities/clip-plan'
import { type ClipProjectDraft } from '@/entities/clip-project'
import {
  CLIP_CAPTION_STYLES,
  CLIP_DEFAULT_CAPTION_STYLE,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
} from '@/entities/clip-design'
import { Checkbox, Typography } from '@/shared/ui'

/** The tile that turns a region off (CLIP-111): it draws nothing, and choosing it keeps the
 *  region's slot draft for when a preset is chosen again (CLIP-189). */
const OFF = 'off'

/** One region's presets as a radiogroup of tiles (CLIP-111, CLIP-165): 사용 안 함 first, then
 *  each preset as the renderer's drawing of its numbered slots on the whole canvas, so an
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
  /** The presets; 사용 안 함 is offered first when the picker can turn the region off. */
  ids: readonly (T | typeof OFF)[]
  value: T | typeof OFF
  samples?: ClipRegionPresetSample[]
  canvas: { width: number; height: number }
  name: (id: T | typeof OFF) => string
  onChange: (next: T | typeof OFF) => void
}) {
  const { t } = useTranslation('clips')
  const label = useId()
  const drawing = new Map<string, string>((samples ?? []).map((s) => [s.preset, s.svg]))
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

/** Whether each region is on, as the server holds it with the owner's pending switch over it,
 *  and the switch itself: the region slots' own save carries it, not the settings (CLIP-111). */
export interface ClipRegionSwitches {
  enabled: Record<'intro' | 'outro', boolean>
  setEnabled: (kind: 'intro' | 'outro', enabled: boolean) => void
}

/** The rest of ①'s design selection (CLIP-111, CLIP-139, CLIP-142): whether the intro is used
 *  and in which preset, the same for the outro, and the caption styles this clip may use. Each
 *  is the project's own and each change re-renders the same plan without a writing call, so
 *  nothing here asks for approval. 사용 안 함 keeps the region's slots; choosing a preset turns
 *  it back on even where the template has no entry for it. */
export function ClipDesignSelection({
  draft,
  onChange,
  regions,
}: {
  projectId: string
  draft: ClipProjectDraft
  onChange: (next: Partial<ClipProjectDraft>) => void
  regions?: ClipRegionSwitches
}) {
  const { t } = useTranslation('clips')
  const samples = useClipLocalStyleSamples(draft.ratio, true, draft.captionPace)
  const drawings = useClipLocalPresetSamples(draft.ratio, t('composition.design.slotLabel'))
  const canvas = CLIP_DESIGN.ratios[draft.ratio].canvas
  const intro = draft.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro
  const outro = draft.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro
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
        ids={regions ? [OFF, ...INTRO_IDS] : INTRO_IDS}
        value={regions && !regions.enabled.intro ? OFF : intro}
        samples={drawings.data?.intro}
        canvas={canvas}
        name={(id) => (id === OFF ? t('project.regionOff') : t(`composition.design.intro_${id}`))}
        onChange={(next) => {
          if (next === OFF) return regions?.setEnabled('intro', false)
          if (next !== intro) onChange({ introPreset: next })
          if (regions && !regions.enabled.intro) regions.setEnabled('intro', true)
        }}
      />
      <PresetPicker
        kind="outro"
        ids={regions ? [OFF, ...OUTRO_IDS] : OUTRO_IDS}
        value={regions && !regions.enabled.outro ? OFF : outro}
        samples={drawings.data?.outro}
        canvas={canvas}
        name={(id) => (id === OFF ? t('project.regionOff') : t(`composition.design.outro_${id}`))}
        onChange={(next) => {
          if (next === OFF) return regions?.setEnabled('outro', false)
          if (next !== outro) onChange({ outroPreset: next })
          if (regions && !regions.enabled.outro) regions.setEnabled('outro', true)
        }}
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
                <ClipCaptionStyleSample fragment={fragment} />
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
