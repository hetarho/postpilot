import { CLIP_SHADOW, CLIP_SPACING, clipPaint } from '@/shared/config'
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
const plateClasses: Record<string, string> = {
  ink_900: 'fill-clip-ink-900',
  paper_50: 'fill-clip-paper-50',
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
  const recipe = copyStyleMeasurements(style)
  const plate = recipe.plate ? clipPaint(recipe.plate as 'ink_900') : null
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
      {plate && (
        <rect
          x="72"
          y="48"
          width="936"
          height="144"
          rx={recipe.radius}
          className={plateClasses[recipe.plate] ?? 'fill-clip-ink-900'}
          fillOpacity={plate.alpha}
        />
      )}
      {accent !== '' && recipe.bar && (
        <rect
          x="72"
          y="48"
          width={CLIP_SPACING.bar_accent}
          height="144"
          className={accentClasses[accent]}
        />
      )}
      {accent !== '' && recipe.dot && (
        <circle
          cx={72 + recipe.padding.h + CLIP_SPACING.dot_accent / 2}
          cy={48 + recipe.padding.v + CLIP_SPACING.dot_accent / 2}
          r={CLIP_SPACING.dot_accent / 2}
          className={accentClasses[accent]}
        />
      )}
      {accented && recipe.highlight && (
        <rect
          x={540 - recipe.fontSize * 1.2}
          y={
            baseline -
            (CLIP_SPACING.underline_mark.raise_em + CLIP_SPACING.underline_mark.height_em) *
              recipe.fontSize
          }
          width={recipe.fontSize * 2.4}
          height={CLIP_SPACING.underline_mark.height_em * recipe.fontSize}
          className={accentClasses[accent]}
          fillOpacity={0.9}
        />
      )}
      <text
        x="540"
        y={baseline}
        textAnchor="middle"
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
