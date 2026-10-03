/** A photo's turn as one of the four quarter turns it may carry (POST-107); anything else — an
 *  absent value from a client older than rotations, or a malformed one — is no turn. */
export type QuarterTurn = 0 | 90 | 180 | 270

export function quarterTurn(rotation: number | undefined): QuarterTurn {
  return rotation === 90 || rotation === 180 || rotation === 270 ? rotation : 0
}

/** The next turn a 회전 press asks for: a quarter clockwise. */
export function nextQuarterTurn(rotation: number | undefined): QuarterTurn {
  return quarterTurn((quarterTurn(rotation) + 90) % 360)
}
