import { clsx } from 'clsx'
import {
  memoryIllustration,
  spokenIllustration,
  templateIllustration,
  voiceIllustration,
} from '@/shared/assets'

const ART = {
  voice: voiceIllustration,
  template: templateIllustration,
  memory: memoryIllustration,
  spoken: spokenIllustration,
} as const

/** Guide: the kit's one set of spot illustrations — one per part an account starts without (말투 ·
 *  템플릿 · 기억 · 목소리), drawn in one hand and one palette (the app's violet, its lavender, the amber
 *  of the home's memory notes) so they sit together and on both themes. Decoration only: the empty
 *  slot it stands in says everything in words. It arrives once and stays still. */
export function KitArt({ part, className }: { part: keyof typeof ART; className?: string }) {
  return (
    <img
      src={ART[part]}
      alt=""
      aria-hidden="true"
      draggable={false}
      className={clsx('animate-rise size-28 object-contain select-none sm:size-32', className)}
    />
  )
}
