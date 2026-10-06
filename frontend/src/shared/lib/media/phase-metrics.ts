export interface MediaPhaseMeasurement {
  samples: number
  totalMs: number
  minMs: number
  maxMs: number
}

export interface MediaPhaseSnapshot<Phase extends string> {
  elapsedMs: number
  phases: Record<Phase, MediaPhaseMeasurement | null>
}

/** Observed wall-clock spans, which may overlap. These are neither CPU/GPU execution
 * times nor an additive decomposition of elapsed time. An unobserved phase is null. */
export class MediaPhaseRecorder<Phase extends string> {
  private readonly started: number
  private readonly values = new Map<Phase, MediaPhaseMeasurement>()
  private readonly allowed: Set<Phase>

  constructor(
    private readonly phases: readonly Phase[],
    private readonly clock: () => number = () => performance.now(),
  ) {
    this.allowed = new Set(phases)
    if (!phases.length || this.allowed.size !== phases.length)
      throw new Error('Invalid media measurement phases')
    this.started = clock()
  }

  begin(phase: Phase) {
    if (!this.allowed.has(phase)) throw new Error('Unknown media measurement phase')
    const started = this.clock()
    let ended = false
    return () => {
      if (ended) return
      ended = true
      const duration = this.clock() - started
      if (!Number.isFinite(duration) || duration < 0)
        throw new Error('Invalid media measurement clock')
      const previous = this.values.get(phase)
      this.values.set(phase, {
        samples: (previous?.samples ?? 0) + 1,
        totalMs: (previous?.totalMs ?? 0) + duration,
        minMs: Math.min(previous?.minMs ?? duration, duration),
        maxMs: Math.max(previous?.maxMs ?? duration, duration),
      })
    }
  }

  measure<Value>(phase: Phase, operation: () => Value): Value {
    const end = this.begin(phase)
    try {
      return operation()
    } finally {
      end()
    }
  }

  async measureAsync<Value>(phase: Phase, operation: () => Promise<Value>): Promise<Value> {
    const end = this.begin(phase)
    try {
      return await operation()
    } finally {
      end()
    }
  }

  /** A lower-level resource adapter may already have observed its wall span. */
  record(phase: Phase, duration: number) {
    if (!this.allowed.has(phase)) throw new Error('Unknown media measurement phase')
    if (!Number.isFinite(duration) || duration < 0)
      throw new Error('Invalid media measurement clock')
    const previous = this.values.get(phase)
    this.values.set(phase, {
      samples: (previous?.samples ?? 0) + 1,
      totalMs: (previous?.totalMs ?? 0) + duration,
      minMs: Math.min(previous?.minMs ?? duration, duration),
      maxMs: Math.max(previous?.maxMs ?? duration, duration),
    })
  }

  snapshot(): MediaPhaseSnapshot<Phase> {
    return {
      elapsedMs: this.clock() - this.started,
      phases: Object.fromEntries(
        this.phases.map((phase) => {
          const value = this.values.get(phase)
          return [phase, value ? { ...value } : null]
        }),
      ) as Record<Phase, MediaPhaseMeasurement | null>,
    }
  }
}
