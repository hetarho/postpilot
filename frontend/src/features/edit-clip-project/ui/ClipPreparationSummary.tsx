import type { TFunction } from 'i18next'
import type { ClipProject, ClipProjectDraft } from '@/entities/clip-project'
import type { ClipComposition, useClipTemplates } from '@/entities/clip-template'
import type { useSpokenLibrary } from '@/entities/spoken-voice'
import { CLIP_DEFAULT_REGION_PRESETS, CLIP_DEFAULT_CAPTION_STYLE } from '@/entities/clip-design'
import type { ClipPreparationRow } from '../model/preparation'
import type { ClipRegionSwitches } from './ClipDesignSelection'

/** Read values from the form's draft, never the project's last autosave response. */
export function clipPreparationRows({
  t,
  draft,
  document,
  inputs,
  selected,
  templates,
  stored,
  regions,
  voiceLibrary,
}: {
  t: TFunction<'clips'>
  draft: ClipProjectDraft
  document: ClipComposition | null | undefined
  inputs: NonNullable<ClipProjectDraft['compositionInputs']>
  selected: ReturnType<typeof useClipTemplates>['templates'][number] | undefined
  templates: ReturnType<typeof useClipTemplates>
  stored?: ClipProject
  regions?: ClipRegionSwitches
  voiceLibrary: ReturnType<typeof useSpokenLibrary>
}): ClipPreparationRow[] {
  const none = t('preparation.optionalEmpty')
  const voice = voiceLibrary.voices.find((v) => v.id === draft.dubbing?.voiceId && !v.removedAt)
  const appearance = t('project.designSummary', {
    intro:
      regions && !regions.enabled.intro
        ? t('project.regionOff')
        : t(`composition.design.intro_${draft.introPreset || CLIP_DEFAULT_REGION_PRESETS.intro}`),
    outro:
      regions && !regions.enabled.outro
        ? t('project.regionOff')
        : t(`composition.design.outro_${draft.outroPreset || CLIP_DEFAULT_REGION_PRESETS.outro}`),
    pace: t(`pace.${draft.captionPace || 'steady'}`),
    accent: t(`accent.${draft.accent || 'none'}`),
    count: draft.allowedCaptionStyles?.length || 1,
  })
  const answers =
    document?.fields.flatMap((field) =>
      field.group
        ? (inputs.items[field.group] ?? []).map((item, index) => ({
            key: `${field.group}-${item.id}-${field.id}`,
            label: `${document.groups.find((g) => g.id === field.group)?.label || field.group} ${index + 1} · ${field.label}`,
            text: item.values[field.id],
          }))
        : [{ key: field.id, label: field.label, text: inputs.values[field.id] }],
    ) ?? []
  return [
    {
      id: 'campaign',
      label: t('project.campaign'),
      screen: 'sources',
      value: (
        <>
          <p>{t(draft.campaign ? 'preparation.campaignOn' : 'preparation.campaignOff')}</p>
          {draft.campaign && (
            <>
              <p className="whitespace-pre-wrap">{draft.advertiserGuide?.text || none}</p>
              <p>
                {t('preparation.keywords', {
                  value: draft.advertiserGuide?.keywords.join(', ') || none,
                })}
              </p>
              <p>
                {t('preparation.hashtags', {
                  value: draft.advertiserGuide?.hashtags.join(', ') || none,
                })}
              </p>
            </>
          )}
        </>
      ),
    },
    {
      id: 'post',
      label: t('linkedPost.title'),
      screen: 'sources',
      value: stored?.linkedPost?.title || t('linkedPost.none'),
    },
    {
      id: 'title',
      label: t('project.name'),
      screen: 'story',
      value: draft.title || t('preparation.titleEmpty'),
    },
    {
      id: 'template',
      label: t('project.template'),
      screen: 'story',
      value: !draft.videoTemplateId
        ? t('project.noTemplate')
        : selected?.name ||
          t(
            templates.isPending
              ? 'project.templateLoading'
              : templates.isError
                ? 'project.templateUnavailable'
                : 'project.detachedTemplate',
          ),
    },
    {
      id: 'answers',
      label: t('preparation.answers'),
      screen: 'story',
      value: answers.length ? (
        <dl className="space-y-2">
          {answers.map((answer) => (
            <div key={answer.key}>
              <dt>{answer.label}</dt>
              <dd className="whitespace-pre-wrap">{answer.text || none}</dd>
            </div>
          ))}
        </dl>
      ) : (
        none
      ),
    },
    {
      id: 'instruction',
      label: t('project.instruction'),
      screen: 'story',
      value: draft.instruction || none,
    },
    {
      id: 'duration',
      label: t('project.duration'),
      screen: 'style',
      value:
        Number.isFinite(draft.targetDurationMs) && draft.targetDurationMs > 0
          ? t('project.summaryDuration', { seconds: draft.targetDurationMs / 1000 })
          : t('preparation.durationEmpty'),
    },
    { id: 'ratio', label: t('project.ratio'), screen: 'style', value: t(`ratio.${draft.ratio}`) },
    {
      id: 'appearance',
      label: t('project.designChoices'),
      screen: 'style',
      value: (
        <>
          <p>{appearance}</p>
          <p>
            {(draft.allowedCaptionStyles?.length
              ? draft.allowedCaptionStyles
              : [CLIP_DEFAULT_CAPTION_STYLE]
            )
              .map((style) => t(`captionStyles.${style as 'bold'}`))
              .join(', ')}
          </p>
          <p>
            {draft.hideDisclosure
              ? t('preparation.disclosureOff')
              : stored?.disclosure
                ? t('composition.retainedDisclosure', {
                    text: t(`disclosure.${stored.disclosure}`),
                  })
                : none}
          </p>
        </>
      ),
    },
    {
      id: 'narration',
      label: t('dubbing.enabled'),
      screen: 'style',
      value: (
        <>
          <p>
            {t(draft.dubbing?.enabled ? 'preparation.narrationOn' : 'preparation.narrationOff')}
          </p>
          {(draft.dubbing?.enabled || draft.dubbing?.voiceId) && (
            <p>
              {voice
                ? t('preparation.voice', { name: voice.name })
                : t(
                    voiceLibrary.voicesQuery.isPending
                      ? 'preparation.voiceLoading'
                      : voiceLibrary.voicesQuery.isError
                        ? 'preparation.voiceFailed'
                        : 'preparation.voiceMissing',
                  )}
            </p>
          )}
        </>
      ),
    },
  ]
}
