import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Brain, ChevronRight, LayoutList, Mic, ScrollText, Shield } from 'lucide-react'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { useTemplates } from '@/entities/template'
import { useGuidelines } from '@/entities/guideline'
import { useMemories } from '@/entities/memory'
import { ListRowIcon, Typography, cardStyles, pageStyles, typographyStyles } from '@/shared/ui'

type Section = 'voices' | 'templates' | 'guidelines' | 'memories' | 'spoken'
const SECTIONS: readonly {
  key: Section
  to: string
  icon: ComponentType<{ className?: string }>
}[] = [
  { key: 'voices', to: '/voices', icon: ScrollText },
  { key: 'guidelines', to: '/guidelines', icon: Shield },
  { key: 'templates', to: '/templates', icon: LayoutList },
  { key: 'memories', to: '/memories', icon: Brain },
  { key: 'spoken', to: '/spoken-voices', icon: Mic },
]

/** Guide: 내 스타일 — "AI에게 알려준 것". Each thing the owner has taught the AI is a big card
 *  named by what it CHANGES (글이 내 말투로 써져요), not by the product's own word for it, with what
 *  is in it right now — the default voice's name, how many rules — so the owner sees their style
 *  before opening anything. */
export function StylePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const voices = useVoices(ownerId)
  const templates = useTemplates(ownerId)
  const guidelines = useGuidelines(ownerId)
  const memories = useMemories(ownerId)
  const detail: Partial<Record<Section, string>> = {
    voices: voices.isPending
      ? undefined
      : voices.defaultVoice
        ? t('guideStyle.defaultVoice', { name: voices.defaultVoice.name })
        : t('guideStyle.voices.count', { count: voices.active.length }),
    templates: templates.isPending
      ? undefined
      : t('guideStyle.templates.count', { count: templates.templates.length }),
    guidelines: guidelines.isPending
      ? undefined
      : t('guideStyle.guidelines.count', { count: guidelines.guidelines.length }),
    memories: memories.isPending
      ? undefined
      : t('guideStyle.memories.count', { count: memories.memories.length }),
  }
  return (
    <main className={pageStyles({ width: 'wide', className: 'pt-4 sm:pt-10' })}>
      <Typography variant="largeTitle">{t('guideStyle.title')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-1">
        {t('guideStyle.lead')}
      </Typography>
      <nav aria-label={t('guideStyle.sections')} className="mt-8">
        <ul className="grid gap-4 sm:grid-cols-2">
          {SECTIONS.map(({ key, to, icon }, index) => (
            <li key={key} className="animate-step-in" style={{ animationDelay: `${index * 50}ms` }}>
              <Link
                to={to}
                className={cardStyles({
                  interactive: true,
                  className: 'flex h-full min-h-28 items-start gap-4',
                })}
              >
                <ListRowIcon icon={icon} size="lg" />
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <Typography variant="title" as="span">
                    {t(`guideStyle.${key}.outcome`)}
                  </Typography>
                  <span className="flex flex-wrap items-center gap-x-2">
                    <span className={typographyStyles({ variant: 'label' })}>
                      {t(`guideStyle.${key}.name`)}
                    </span>
                    {detail[key] && (
                      <span className={typographyStyles({ variant: 'meta' })}>{detail[key]}</span>
                    )}
                  </span>
                </span>
                <ChevronRight
                  aria-hidden="true"
                  className="text-content-tertiary mt-1 size-6 shrink-0"
                />
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </main>
  )
}
