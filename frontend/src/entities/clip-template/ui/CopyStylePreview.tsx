import { COPY_STYLE_MEASUREMENTS, type ClipAccent, type CopyStyle } from '../model/types'

const accentClasses: Record<ClipAccent, string> = {
  '': 'fill-clip-white',
  coral: 'fill-clip-coral',
  amber: 'fill-clip-amber',
  lime: 'fill-clip-lime',
  teal: 'fill-clip-teal',
  blue: 'fill-clip-blue',
  violet: 'fill-clip-violet',
  pink: 'fill-clip-pink',
}

/** A static crop of the renderer's 1080-wide canvas, with exact text rather than generated art. */
export function CopyStylePreview({
  style,
  accent = '',
  text,
}: {
  style: CopyStyle
  accent?: ClipAccent
  text: string
}) {
  const recipe = COPY_STYLE_MEASUREMENTS[style]
  return (
    <svg
      viewBox="0 0 1080 240"
      role="img"
      aria-label={text}
      className="mt-3 w-full rounded-md"
      data-copy-style={style}
    >
      <rect width="1080" height="240" className="fill-surface-recessed" />
      {style !== 'emphasis' && (
        <rect
          x="72"
          y="48"
          width="936"
          height="144"
          rx={recipe.radius}
          className={style === 'diary' ? 'fill-clip-paper' : 'fill-clip-ink'}
          fillOpacity={style === 'clean' ? 0.78 : 1}
        />
      )}
      {accent && (
        <rect x="90" y="80" width="8" height="80" rx="4" className={accentClasses[accent]} />
      )}
      <text
        x="540"
        y="122"
        textAnchor="middle"
        dominantBaseline="middle"
        fontSize={recipe.fontSize}
        fontWeight={recipe.weight}
        className={
          style === 'diary'
            ? 'fill-clip-ink'
            : style === 'emphasis'
              ? 'fill-clip-white stroke-clip-ink'
              : 'fill-clip-white'
        }
        strokeWidth={style === 'emphasis' ? 8 : 0}
        paintOrder="stroke fill"
      >
        {text}
      </text>
    </svg>
  )
}
