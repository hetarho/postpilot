import type { ComponentType } from 'react'
import { AudioLines, Brain, LayoutList, PenTool, Quote, ShieldCheck } from 'lucide-react'
import type { KitPartKey } from '../model/readiness'

type Icon = ComponentType<{ className?: string; 'aria-hidden'?: boolean | 'true' }>
export const KIT_ICONS: Record<KitPartKey, Icon> = {
  voice: Quote,
  template: LayoutList,
  guideline: ShieldCheck,
  memory: Brain,
  model: PenTool,
  spoken: AudioLines,
}
