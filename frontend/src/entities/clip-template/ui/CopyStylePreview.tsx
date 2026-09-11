import {
  COPY_STYLE_MEASUREMENTS,
  PLATED_COPY_STYLES,
  type ClipAccent,
  type CopyStyle,
} from '../model/types'

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
      {PLATED_COPY_STYLES.includes(style) && (
        <rect
          x="72"
          y="48"
          width="936"
          height="144"
          rx={recipe.radius}
          className={style === 'memo' ? 'fill-clip-paper' : 'fill-clip-ink'}
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
          style === 'memo'
            ? 'fill-clip-ink'
            : !PLATED_COPY_STYLES.includes(style)
              ? 'fill-clip-white stroke-clip-ink'
              : 'fill-clip-white'
        }
        strokeWidth={PLATED_COPY_STYLES.includes(style) ? 0 : 8}
        paintOrder="stroke fill"
      >
        {text}
      </text>
    </svg>
  )
}
