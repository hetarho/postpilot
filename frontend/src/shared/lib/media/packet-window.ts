/** Encoded callbacks cannot await. Reserve encoder capacity before submission,
 * then acknowledge each packet only after its async sink accepts it. */
let nextPacketId = 0
export class MediaPacketWindow {
  private pending = new Map<number, number>()
  private waiters = new Set<() => void>()
  private failure: unknown
  private bytes = 0
  private peakPackets = 0
  private peakBytes = 0
  private reservations = 0
  constructor(
    private limits: { packets: number; bytes: number; timeoutMs: number },
    private signal: AbortSignal,
  ) {}
  send(bytes: number, publish: (id: number) => void) {
    this.signal.throwIfAborted()
    if (this.failure) throw this.failure
    if (this.reservations) this.reservations--
    if (
      this.pending.size >= this.limits.packets ||
      bytes <= 0 ||
      bytes + this.bytes > this.limits.bytes
    )
      throw new Error('MEDIA_PACKET_WINDOW_LIMIT')
    const id = ++nextPacketId
    this.pending.set(id, bytes)
    this.bytes += bytes
    this.peakPackets = Math.max(this.peakPackets, this.pending.size)
    this.peakBytes = Math.max(this.peakBytes, this.bytes)
    try {
      publish(id)
    } catch (error) {
      this.ack(id, error)
      throw error
    }
  }
  ack(id: number, error?: unknown) {
    const bytes = this.pending.get(id)
    if (bytes === undefined) return
    this.pending.delete(id)
    this.bytes -= bytes
    if (error) this.failure ??= error
    for (const wake of this.waiters) wake()
  }
  async capacity(reserve = 0) {
    await this.wait(() => this.pending.size + reserve < this.limits.packets)
  }
  async reserve() {
    await this.wait(() => {
      if (this.pending.size + this.reservations >= this.limits.packets) return false
      this.reservations++
      return true
    })
  }
  async drain() {
    await this.wait(() => !this.pending.size && !this.reservations)
  }
  private async wait(ready: () => boolean) {
    this.signal.throwIfAborted()
    if (this.failure) throw this.failure
    if (ready()) return
    await new Promise<void>((resolve, reject) => {
      const finish = (error?: unknown) => {
        clearTimeout(timer)
        this.waiters.delete(wake)
        this.signal.removeEventListener('abort', abort)
        if (error) reject(error)
        else resolve()
      }
      const wake = () => {
        if (this.failure) finish(this.failure)
        else if (ready()) finish()
      }
      const abort = () => finish(this.signal.reason)
      const timer = setTimeout(
        () => finish(new Error('MEDIA_PACKET_SINK_TIMEOUT')),
        this.limits.timeoutMs,
      )
      this.waiters.add(wake)
      this.signal.addEventListener('abort', abort, { once: true })
      if (this.signal.aborted) abort()
      else wake()
    })
  }
  measurements() {
    return { peakPackets: this.peakPackets, peakBytes: this.peakBytes }
  }
}
