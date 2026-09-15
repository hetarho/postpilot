import { CLIP_DESIGN, CLIP_SHADOW } from '@/shared/config'
import { copyStyleMeasurements, type ClipAccent, type CopyStyle } from '../model/types'

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
/** A static crop of the renderer's 1080-wide canvas, with exact text rather than
 *  generated art. Every size, radius, opacity, stroke and offset comes from the
 *  design system the renderer embeds (`shared/config/clip-design.json`), so the
 *  preview cannot drift from what a render actually draws. */
export function CopyStylePreview({
  style,
  accent = '',
  keyword = '',
  text,
}: {
  style: CopyStyle
  accent?: ClipAccent
  /** The one word 크게 강조 colours and 형광펜 highlights (CDS-25, CDS-26). */
  keyword?: string
  text: string
}) {
  const recipe = copyStyleMeasurements()
  const accented = accent !== '' && (recipe.highlight || recipe.stroke > 0) && keyword !== ''
  const baseline = 120 + recipe.fontSize / 3
  return (
    <svg
      viewBox="0 0 1080 240"
      role="img"
      aria-label={text}
      className="mt-3 w-full rounded-md"
      data-copy-style={style}
    >
      {recipe.shadow && (
        <defs>
          <filter id={`clip-shadow-${style}`} x="-20%" y="-20%" width="140%" height="140%">
            <feDropShadow
              dx={CLIP_SHADOW.text.dx}
              dy={CLIP_SHADOW.text.dy}
              stdDeviation={CLIP_SHADOW.text.blur / 2}
              floodColor={CLIP_SHADOW.text.hex}
              floodOpacity={CLIP_SHADOW.text.alpha}
            />
          </filter>
        </defs>
      )}
      <rect width="1080" height="240" className="fill-surface-recessed" />
      <text
        x="540"
        y={baseline}
        textAnchor="middle"
        fontFamily={CLIP_DESIGN.faces.paperlogy}
        fontSize={recipe.fontSize}
        fontWeight={recipe.weight}
        letterSpacing={recipe.tracking * recipe.fontSize}
        className={
          recipe.plate === 'paper_50'
            ? 'fill-clip-ink'
            : recipe.stroke > 0
              ? 'fill-clip-white stroke-clip-stroke'
              : 'fill-clip-white'
        }
        strokeWidth={recipe.stroke}
        strokeLinejoin="round"
        paintOrder="stroke fill"
        filter={recipe.shadow ? `url(#clip-shadow-${style})` : undefined}
      >
        {accented && !recipe.highlight ? (
          <>
            {text.split(keyword)[0]}
            <tspan className={accentClasses[accent]} strokeWidth={recipe.stroke}>
              {keyword}
            </tspan>
            {text.split(keyword).slice(1).join(keyword)}
          </>
        ) : (
          text
        )}
      </text>
    </svg>
  )
}
