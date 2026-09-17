import { useTranslation } from 'react-i18next'
import { CompositionDesignThumbnail } from '@/entities/clip-template'
import {
  useClipCaptionStyleSamples,
  type ClipCaptionFragment,
  type ClipProjectDraft,
} from '@/entities/clip-project'
import { CLIP_CAPTION_STYLES, CLIP_DEFAULT_CAPTION_STYLE } from '@/shared/config'
import { Checkbox, SegmentedControl, Typography } from '@/shared/ui'

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
      <Typography variant="fieldTitle" as="p">
        {t('composition.design.intro')}
      </Typography>
      <SegmentedControl
        ariaLabel={t('composition.design.intro')}
        value={draft.introPreset || 'b'}
        options={(['a', 'b'] as const).map((value) => ({
          value,
          label: t(`composition.design.intro_${value}`),
          preview: <CompositionDesignThumbnail kind="intro" value={value} />,
        }))}
        onChange={(introPreset) => onChange({ introPreset })}
      />
      <Typography variant="fieldTitle" as="p">
        {t('composition.design.outro')}
      </Typography>
      <SegmentedControl
        ariaLabel={t('composition.design.outro')}
        value={draft.outroPreset || 'e'}
        options={(['b', 'e'] as const).map((value) => ({
          value,
          label: t(`composition.design.outro_${value}`),
          preview: <CompositionDesignThumbnail kind="outro" value={value} />,
        }))}
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
