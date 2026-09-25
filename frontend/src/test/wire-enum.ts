// The fakes' own mapping between a domain id and its proto enum member, derived from the member
// name alone: `domestic_travel` is `DOMESTIC_TRAVEL`. A fake that reused the entity's mapper could
// not catch that mapper's bug, so the harness spells the rule itself (ARCH-17).

type WireEnum = Record<string | number, string | number>

/** The member an id names. Throws when it names none: a fixture spelled wrong is the test's bug. */
export function toWire(wire: WireEnum, id: string): number {
  const value = wire[id.toUpperCase()]
  if (typeof value !== 'number') throw new Error(`fake: ${id} names no member`)
  return value
}

/** The id a member number reads as, or undefined for UNSPECIFIED and a number no id names. */
export function fromWire<T extends string>(
  wire: WireEnum,
  value: number,
  ids: readonly T[],
): T | undefined {
  const name = wire[value]
  if (typeof name !== 'string') return undefined
  const id = name.toLowerCase()
  return (ids as readonly string[]).includes(id) ? (id as T) : undefined
}
