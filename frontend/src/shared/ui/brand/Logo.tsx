import { twMerge } from 'tailwind-merge'

/** Stroke settings shared by the glyphs. The source artwork paints a 12-unit stroke of the glyph's
 *  own colour UNDER the fill (`paint-order`), which is what rounds the terminals and gives the
 *  wordmark its slightly soft weight. The source file also carried `vector-effect:
 *  non-scaling-stroke` — deliberately dropped: it pins the stroke to 12 *screen* pixels, so at the
 *  28px the header renders this at, the letters weld into a blob. Scaling with the viewBox is what
 *  makes one file work from 28px to a hero. */
const GLYPH_STROKE = {
  strokeWidth: 12,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
  paintOrder: 'stroke fill',
} as const

/** The Postpilot wordmark — `post` in the page's own ink, `pilot` in the accent, with the paper
 *  plane standing in for the dot of the i.
 *
 *  Inline SVG rather than an `<img src="/logo.svg">` on purpose: only inline markup can take
 *  `currentColor` and the `brand-*` utilities, so the logo re-skins with the theme (design-language
 *  §2.4) instead of freezing one palette into a file that will be wrong the day the day/night
 *  switcher lands. It also costs no request and cannot flash in late above the fold.
 *
 *  The viewBox is trimmed to the ink (plus ~14 units so the round terminals and the descender are
 *  not clipped), so the caller positions the WORD, not a square of empty canvas. Size it by height
 *  — `h-7 w-auto` — and the aspect ratio does the rest. */
export function Logo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="167 794 1713 461"
      role="img"
      aria-label="Postpilot"
      // `block` because an inline SVG sits on the text baseline and drags ~4px of descender
      // space into every flex row it lands in.
      className={twMerge('block h-7 w-auto', className)}
    >
      <g className="fill-brand-wordmark stroke-brand-wordmark" {...GLYPH_STROKE}>
        <path
          d="M166 -163Q166 -174 158.0 -182.0Q150 -190 139 -190H102Q91 -190 83.0 -182.0Q75 -174 75 -163V493Q75 504 83.0 512.0Q91 520 102 520H139Q150 520 158.0 512.0Q166 504 166 493V462Q217 530 321 530Q383 530 433.5 501.0Q484 472 514.5 418.0Q545 364 548 292Q549 282 549 260Q549 237 548 227Q545 155 514.5 101.5Q484 48 434.0 19.0Q384 -10 321 -10Q265 -10 226.5 9.5Q188 29 166 58ZM311 76Q376 76 414.5 118.0Q453 160 457 232Q458 242 458 260Q458 278 457 288Q453 360 414.5 402.0Q376 444 311 444Q249 444 209.0 405.5Q169 367 166 303L165 264L166 224Q169 154 208.0 115.0Q247 76 311 76Z"
          transform="matrix(0.4683544 0 0 -0.4305556 151.8734 1153.1944)"
        />
        <path
          d="M527 259Q527 235 525 213Q516 111 454.5 50.5Q393 -10 288 -10Q183 -10 121.5 50.5Q60 111 51 213Q50 224 50 259Q50 296 51 307Q59 409 121.0 469.5Q183 530 288 530Q393 530 455.0 469.5Q517 409 525 307Q527 285 527 259ZM288 444Q221 444 184.5 405.0Q148 366 142 302Q141 290 141 259Q141 229 142 218Q148 154 184.5 115.0Q221 76 288 76Q355 76 391.5 115.0Q428 154 434 218Q436 240 436 259Q436 278 434 302Q428 366 391.5 405.0Q355 444 288 444Z"
          transform="matrix(0.4654088 0 0 -0.4055556 406.7296 1139.9444)"
        />
        <path
          d="M54 380Q54 421 76.0 455.0Q98 489 141.5 509.5Q185 530 248 530Q310 530 355.0 512.0Q400 494 423.5 467.0Q447 440 447 415Q447 405 439.0 397.5Q431 390 420 390H387Q368 390 357 404Q326 444 248 444Q202 444 173.5 427.0Q145 410 145 380Q145 355 159.0 342.0Q173 329 201.0 321.0Q229 313 300 297Q388 278 425.0 239.0Q462 200 462 144Q462 103 436.5 68.0Q411 33 363.5 11.5Q316 -10 253 -10Q191 -10 143.5 9.5Q96 29 70.0 57.0Q44 85 44 110Q44 120 52.0 127.5Q60 135 71 135H107Q116 135 123.0 133.0Q130 131 136 122Q169 76 253 76Q300 76 335.5 95.5Q371 115 371 144Q371 168 353.5 182.5Q336 197 303.0 206.5Q270 216 200 231Q54 262 54 380Z"
          transform="matrix(0.4330144 0 0 -0.4055556 652.9474 1139.9444)"
        />
        <path
          d="M295 86H348Q359 86 367.0 78.0Q375 70 375 59V27Q375 16 367.0 8.0Q359 0 348 0H285Q200 0 160.0 44.5Q120 89 120 175V434H46Q35 434 27.0 442.0Q19 450 19 461V493Q19 504 27.0 512.0Q35 520 46 520H120V683Q120 694 128.0 702.0Q136 710 147 710H184Q195 710 203.0 702.0Q211 694 211 683V520H338Q349 520 357.0 512.0Q365 504 365 493V461Q365 450 357.0 442.0Q349 434 338 434H211V175Q211 132 228.5 109.0Q246 86 295 86Z"
          transform="matrix(0.3820225 0 0 -0.3746479 864.7416 1140.0000)"
        />
      </g>
      <g className="fill-brand-mark">
        {/* Only the letters carry the stroke; the two stems and the plane are drawn at their
            finished weight, exactly as in the source artwork. */}
        <g className="stroke-brand-mark" {...GLYPH_STROKE}>
          <path
            d="M166 -163Q166 -174 158.0 -182.0Q150 -190 139 -190H102Q91 -190 83.0 -182.0Q75 -174 75 -163V493Q75 504 83.0 512.0Q91 520 102 520H139Q150 520 158.0 512.0Q166 504 166 493V462Q217 530 321 530Q383 530 433.5 501.0Q484 472 514.5 418.0Q545 364 548 292Q549 282 549 260Q549 237 548 227Q545 155 514.5 101.5Q484 48 434.0 19.0Q384 -10 321 -10Q265 -10 226.5 9.5Q188 29 166 58ZM311 76Q376 76 414.5 118.0Q453 160 457 232Q458 242 458 260Q458 278 457 288Q453 360 414.5 402.0Q376 444 311 444Q249 444 209.0 405.5Q169 367 166 303L165 264L166 224Q169 154 208.0 115.0Q247 76 311 76Z"
            transform="matrix(0.4683544 0 0 -0.4305556 1006.8734 1153.1944)"
          />
          <path
            d="M527 259Q527 235 525 213Q516 111 454.5 50.5Q393 -10 288 -10Q183 -10 121.5 50.5Q60 111 51 213Q50 224 50 259Q50 296 51 307Q59 409 121.0 469.5Q183 530 288 530Q393 530 455.0 469.5Q517 409 525 307Q527 285 527 259ZM288 444Q221 444 184.5 405.0Q148 366 142 302Q141 290 141 259Q141 229 142 218Q148 154 184.5 115.0Q221 76 288 76Q355 76 391.5 115.0Q428 154 434 218Q436 240 436 259Q436 278 434 302Q428 366 391.5 405.0Q355 444 288 444Z"
            transform="matrix(0.4654088 0 0 -0.4055556 1463.7296 1139.9444)"
          />
          <path
            d="M295 86H348Q359 86 367.0 78.0Q375 70 375 59V27Q375 16 367.0 8.0Q359 0 348 0H285Q200 0 160.0 44.5Q120 89 120 175V434H46Q35 434 27.0 442.0Q19 450 19 461V493Q19 504 27.0 512.0Q35 520 46 520H120V683Q120 694 128.0 702.0Q136 710 147 710H184Q195 710 203.0 702.0Q211 694 211 683V520H338Q349 520 357.0 512.0Q365 504 365 493V461Q365 450 357.0 442.0Q349 434 338 434H211V175Q211 132 228.5 109.0Q246 86 295 86Z"
            transform="matrix(0.3848315 0 0 -0.3746479 1715.6882 1140.0000)"
          />
        </g>
        <rect x="1295" y="929" width="60" height="211" rx="10" />
        <rect x="1396" y="855" width="60" height="285" rx="10" />
        <path d="M 1280 838 L 1348 812 C 1358 808 1367 817 1363 828 L 1339 894 C 1336 904 1327 906 1323 896 L 1311 865 L 1278 854 C 1268 851 1269 842 1280 838 Z" />
      </g>
    </svg>
  )
}
