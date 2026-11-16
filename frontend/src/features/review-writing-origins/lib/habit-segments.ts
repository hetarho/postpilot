import type { OriginFieldLocator } from '@/entities/post'
import type { VoiceHabitPhrase } from '@/entities/voice'

export interface HabitSegment {
  text: string
  start: number
  end: number
  habit?: VoiceHabitPhrase
}

/** Splits one field's text into plain runs and the habit phrases found in it, by half-open
 *  Unicode scalar offsets as origin spans are. A phrase that overlaps an earlier one, or whose
 *  quote no longer matches the text at its offsets, is left plain rather than drawn misplaced. */
export function habitSegments(
  text: string,
  field: OriginFieldLocator,
  habits: readonly VoiceHabitPhrase[],
): HabitSegment[] {
  const chars = Array.from(text)
  const key = JSON.stringify(field)
  const found = habits
    .filter(
      (habit) =>
        JSON.stringify(habit.field) === key &&
        habit.start < habit.end &&
        habit.end <= chars.length &&
        chars.slice(habit.start, habit.end).join('') === habit.quote,
    )
    .sort((a, b) => a.start - b.start)
  const segments: HabitSegment[] = []
  let at = 0
  for (const habit of found) {
    if (habit.start < at) continue
    if (habit.start > at)
      segments.push({ text: chars.slice(at, habit.start).join(''), start: at, end: habit.start })
    segments.push({ text: habit.quote, start: habit.start, end: habit.end, habit })
    at = habit.end
  }
  if (at < chars.length || segments.length === 0)
    segments.push({ text: chars.slice(at).join(''), start: at, end: chars.length })
  return segments
}
