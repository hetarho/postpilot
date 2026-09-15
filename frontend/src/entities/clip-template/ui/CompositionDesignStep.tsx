import { useTranslation } from 'react-i18next'
import { CLIP_COMPOSITION_PREVIEW } from '@/shared/config'
import { SegmentedControl, Typography } from '@/shared/ui'
import { compositionSkeleton, type CompositionDesign } from '../lib/composition-skeleton'
import { parseClipComposition } from '../lib/composition-parse'
import { sampleClipComposition } from '../lib/composition-sample'
import { CompositionDesignFrame } from './CompositionDesignFrame'

function Thumbnail({ kind, value }: { kind: keyof CompositionDesign; value: string }) {
  const { t } = useTranslation('clips')
  const source = compositionSkeleton(
    kind === 'intro' && value === 'a' ? 'a' : 'b',
    kind === 'outro' && value === 'b' ? 'b' : 'e',
  )
  const document = parseClipComposition(source)
  const role = kind === 'intro' ? 'hook' : kind === 'outro' ? 'ending' : 'caption'
  // Use the exact representative drawing with generated sample words in every preset slot.
  document.elements = document.elements.map((e) => ({
    ...e,
    kind: 'ai',
    rows: e.rows.map((r) => ({
      ...r,
      kind: 'ai',
      parts: [{ literal: t('composition.sampleShortAI'), field: '' }],
    })),
  }))
  if (kind === 'caption') {
    const caption = parseClipComposition(
      source.replace(
        '</clip>',
        '<text id="sample-caption" role="caption" kind="ai" basis="whole">Scene</text></clip>',
      ),
    ).elements.find((e) => e.role === 'caption')!
    document.elements.push(caption)
  }
  const timeline = sampleClipComposition(document, CLIP_COMPOSITION_PREVIEW.durationMs, () =>
    t('composition.sampleShortAI'),
  )
  return (
    <div className="mx-auto mb-2 w-24" aria-hidden="true">
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements.filter((e) => e.element.role === role)}
        ratio="vertical"
        label={t(`composition.design.${kind}`)}
        sampleAI={t('composition.sampleShortAI')}
      />
    </div>
  )
}

export function CompositionDesignStep({
  value,
  onChange,
}: {
  value: Partial<CompositionDesign>
  onChange: (design: Partial<CompositionDesign>) => void
}) {
  const { t } = useTranslation('clips')
  const options = { intro: ['a', 'b'], caption: ['bold'], outro: ['b', 'e'] } as const
  return (
    <section className="min-w-0 space-y-4" aria-label={t('composition.design.title')}>
      <Typography variant="fieldTitle" as="h2">
        {t('composition.design.title')}
      </Typography>
      <Typography variant="body">{t('composition.design.help')}</Typography>
      {(['intro', 'caption', 'outro'] as const).map((kind) => (
        <div key={kind} className="min-w-0 space-y-2">
          <Typography variant="fieldTitle">{t(`composition.design.${kind}`)}</Typography>
          <SegmentedControl
            value={value[kind] ?? ''}
            ariaLabel={t(`composition.design.${kind}`)}
            options={options[kind].map((option) => ({
              value: option,
              label: t(`composition.design.${kind}_${option}`, { defaultValue: option }),
              preview: <Thumbnail kind={kind} value={option} />,
            }))}
            onChange={(option) => onChange({ ...value, [kind]: option })}
          />
        </div>
      ))}
    </section>
  )
}
