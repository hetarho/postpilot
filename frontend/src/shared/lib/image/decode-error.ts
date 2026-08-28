/** Why a file could not be turned into pixels, in terms the skipped list can explain.
 *
 *  `heif-unsupported` is the device's failure (PRD §7: a phone that cannot run the
 *  decoder), `unreadable` is the file's. */
export type DecodeFailure = 'unreadable' | 'heif-unsupported'

export class DecodeError extends Error {
  constructor(
    readonly reason: DecodeFailure,
    detail?: string,
  ) {
    super(detail ?? reason)
    this.name = 'DecodeError'
  }
}
