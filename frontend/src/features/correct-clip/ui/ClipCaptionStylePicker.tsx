import { RadioTile } from '@/shared/ui'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipCaptionStyleSample, type ClipCaptionFragment } from '@/entities/clip-plan'
import { CLIP_CAPTION_STYLES } from '@/entities/clip-design'
import { radioTiles, Typography } from '@/shared/ui'

/** One caption's style, chosen from EVERY approved style (CLIP-142, CLIP-143): the project's AI
 *  set bounds what a writer picks, never the owner. Each tile is the renderer's own drawing of
 *  the style (CDS-83) above its name; the first is the caption's own style — the one its plan
 *  names, or the AI set's first — drawn as it is now. While the drawings load, or when they
 *  fail, the tiles show names and stay selectable. */
export function ClipCaptionStylePicker({
  value,
  drawn,
  samples,
  unavailable,
  onChange,
}: {
  /** The owner's own choice; absent while the caption takes its plan's style. */
  value?: string
  /** The style the caption is drawn in when the owner chose none. */
  drawn: string
  samples?: readonly ClipCaptionFragment[]
  unavailable?: boolean
  onChange: (style: string | undefined) => void
}) {
  const { t } = useTranslation('clips')
  const label = useId()
  const byStyle = new Map((samples ?? []).map((sample) => [sample.style, sample]))
  const options: { id: string | undefined; style: string; name: string }[] = [
    { id: undefined, style: drawn, name: t('placement.styleDefault') },
    ...CLIP_CAPTION_STYLES.map((id) => ({ id, style: id, name: t(`captionStyles.${id}`) })),
  ]
  const chosen = Math.max(
    0,
    options.findIndex((option) => option.id === value),
  )
  const tiles = radioTiles(options.length, chosen, (index) => onChange(options[index].id))
  return (
    <div {...tiles.group} aria-labelledby={label} className="min-w-0">
      <Typography id={label} variant="fieldTitle" as="p">
        {t('placement.style')}
      </Typography>
      <div className="mt-2 grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-3">
        {options.map((option, index) => {
          const sample = byStyle.get(option.style)
          return (
            <RadioTile
              key={option.id ?? ''}
              data-style={option.id ?? ''}
              {...tiles.tile(index, 'min-h-11 gap-1')}
            >
              <span className="bg-media-canvas-bg pointer-events-none flex w-full items-center rounded-md p-2">
                <ClipCaptionStyleSample fragment={sample} className="h-12 w-full" />
              </span>
              <Typography variant="body" as="span" className="min-w-0 break-words">
                {option.name}
              </Typography>
              {sample?.representativeFrame && (
                <Typography variant="meta" as="span" className="text-content-secondary">
                  {t('placement.perFrame')}
                </Typography>
              )}
            </RadioTile>
          )
        })}
      </div>
      {unavailable && (
        <Typography variant="meta" className="text-content-secondary mt-2">
          {t('placement.samplesUnavailable')}
        </Typography>
      )}
    </div>
  )
}
