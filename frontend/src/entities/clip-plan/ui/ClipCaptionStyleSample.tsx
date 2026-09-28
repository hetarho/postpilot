import type { ClipCaptionFragment } from '../api/caption-preview'

/** One caption style as the RENDERER draws it, scaled into the row (CDS-83). The fragment is
 *  already at the origin, so its own box is the whole viewBox; while it is missing — loading,
 *  or a sample that could not be drawn — nothing stands in for it. */
export function ClipCaptionStyleSample({
  fragment,
  className = 'h-8 w-full',
}: {
  fragment?: ClipCaptionFragment
  className?: string
}) {
  if (!fragment || fragment.box.width <= 0 || fragment.box.height <= 0) return null
  return (
    <svg
      viewBox={`0 0 ${fragment.box.width} ${fragment.box.height}`}
      className={className}
      preserveAspectRatio="xMinYMid meet"
      aria-hidden="true"
      /* The server's own drawing of this style: there is no other way to show the very SVG
         the renderer produces (CDS-83). */
      dangerouslySetInnerHTML={{ __html: fragment.svg }}
    />
  )
}
