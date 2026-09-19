/** The aurora shader's frame budget. Thirty frames a second is more than a slow drift needs and
 *  half what a display will ask for, so the stage costs battery like a video, not like a game. */
export const PROMO_AURORA_MIN_FRAME_MS = 33
/** The shader renders at this fraction of the stage's CSS pixels: the picture is smooth noise
 *  the browser scales up, so full resolution would buy nothing visible for four times the work. */
export const PROMO_AURORA_RESOLUTION_SCALE = 0.5
