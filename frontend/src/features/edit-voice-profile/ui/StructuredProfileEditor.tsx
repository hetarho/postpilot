import { useTranslation } from 'react-i18next'
import type { VoiceAxes, VoiceProfile } from '@/entities/voice'
import { type ContentLanguage, VoiceLayer } from '@/shared/api'
import { formatNumber, formatPercent } from '@/shared/lib'
import { Badge, Typography, typographyStyles } from '@/shared/ui'
import { ProfileField } from './ProfileField'

/** The six axes in their canonical order. They are listed explicitly rather than iterated from the
 *  object, because an axis the analysis never answered is ABSENT — iterating its keys would drop
 *  the unknown ones from the screen instead of reporting them. */
export function StructuredProfileEditor({
  ownerId,
  voiceId,
  profile,
  sourceLanguage,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  sourceLanguage: ContentLanguage
  readOnly?: boolean
}) {
  const { t } = useTranslation('voices')
  const structured = profile.structured
  const axes: ReadonlyArray<{ key: keyof VoiceAxes; label: string }> = [
    { key: 'involvement', label: t('profile.axis.involvement') },
    { key: 'narrativity', label: t('profile.axis.narrativity') },
    { key: 'persuasionOvertness', label: t('profile.axis.persuasionOvertness') },
    { key: 'abstractness', label: t('profile.axis.abstractness') },
    { key: 'addresseeFocus', label: t('profile.axis.addresseeFocus') },
    { key: 'humor', label: t('profile.axis.humor') },
  ]
  const fields = [
    {
      label: t('profile.field.lexical'),
      layer: VoiceLayer.LEXICAL,
      field: 'description',
      value: structured.lexical.description,
    },
    {
      label: t(`profile.field.${sourceLanguage === 'en' ? 'endingEn' : 'endingKo'}`),
      layer: VoiceLayer.ENDINGS,
      field: 'base_register',
      value: structured.endings.baseRegister,
    },
    {
      label: t('profile.field.connective'),
      layer: VoiceLayer.SYNTAX,
      field: 'connective_style',
      value: structured.syntax.connectiveStyle,
    },
    {
      label: t('profile.field.intro'),
      layer: VoiceLayer.STRUCTURE,
      field: 'intro_pattern',
      value: structured.structure.introPattern,
    },
    {
      label: t('profile.field.closing'),
      layer: VoiceLayer.STRUCTURE,
      field: 'closing_pattern',
      value: structured.structure.closingPattern,
    },
  ]
  return (
    <section aria-labelledby="profile-heading">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Typography variant="title" as="h3" id="profile-heading">
          {t('profile.current')}
        </Typography>
        <Typography variant="label" className="text-content-tertiary">
          {t('profile.finalizedCount', {
            version: structured.version.toString(),
            count: profile.finalizedSourceCount,
          })}
        </Typography>
      </div>
      {structured.empty ? (
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('profile.empty')}
        </Typography>
      ) : (
        <div className="mt-5 space-y-6">
          <div className="grid gap-4 sm:grid-cols-2">
            {fields.map((item) => (
              <ProfileField
                key={`${item.layer}-${item.field}`}
                ownerId={ownerId}
                voiceId={voiceId}
                readOnly={readOnly}
                {...item}
              />
            ))}
          </div>
          <section>
            <Typography variant="title" as="h4">
              {t('profile.endings')}
            </Typography>
            <div className="mt-2 flex flex-wrap gap-2">
              {structured.endings.distribution.map((item) => (
                <Badge key={item.ending}>
                  {t('profile.endingShare', {
                    ending: item.ending,
                    rate: formatPercent(item.ratio),
                  })}
                </Badge>
              ))}
            </div>
          </section>
          <section>
            <Typography variant="title" as="h4">
              {t('profile.sentenceStructure')}
            </Typography>
            <dl
              className={typographyStyles({
                variant: 'label',
                className: 'mt-2 grid gap-2 sm:grid-cols-2',
              })}
            >
              <div>
                <dt className="text-content-tertiary">
                  {sourceLanguage === 'en'
                    ? t('profile.averageSentenceWords')
                    : t('profile.averageSentenceChars')}
                </dt>
                {sourceLanguage === 'en' ? (
                  <dd
                    className={
                      structured.syntax.averageSentenceWords === undefined
                        ? 'text-content-tertiary'
                        : undefined
                    }
                  >
                    {structured.syntax.averageSentenceWords === undefined
                      ? t('profile.unknown')
                      : t('profile.words', {
                          count: structured.syntax.averageSentenceWords,
                          formatted: formatNumber(structured.syntax.averageSentenceWords),
                        })}
                  </dd>
                ) : (
                  <dd>
                    {t('profile.characters', {
                      count: structured.syntax.averageSentenceChars,
                      formatted: formatNumber(structured.syntax.averageSentenceChars),
                    })}
                  </dd>
                )}
              </div>
              <div>
                <dt className="text-content-tertiary">{t('profile.sentencesPerParagraph')}</dt>
                <dd>
                  {t('profile.rangeCount', {
                    min: formatNumber(structured.structure.paragraphSentencesMin),
                    max: formatNumber(structured.structure.paragraphSentencesMax),
                  })}
                </dd>
              </div>
            </dl>
          </section>
          <section>
            <Typography variant="title" as="h4">
              {t('profile.tendencies')}
            </Typography>
            <dl
              className={typographyStyles({
                variant: 'label',
                className: 'mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3',
              })}
            >
              {axes.map((axis) => {
                const value = structured.axes[axis.key]
                return (
                  <div key={axis.key}>
                    <dt className="text-content-tertiary">{axis.label}</dt>
                    <dd className={value === undefined ? 'text-content-tertiary' : undefined}>
                      {value === undefined ? t('profile.unknown') : formatNumber(value)}
                    </dd>
                  </div>
                )
              })}
            </dl>
          </section>
          {(structured.lexical.bannedWords.length > 0 ||
            structured.lexical.bannedPatterns.length > 0 ||
            structured.endings.bannedEndings.length > 0) && (
            <section>
              <Typography variant="title" as="h4">
                {t('profile.avoidExpressions')}
              </Typography>
              <ul
                className={typographyStyles({ variant: 'label', className: 'mt-2 list-disc pl-5' })}
              >
                {structured.lexical.bannedWords.map((v) => (
                  <li key={v.value}>
                    {v.value}
                    {v.reason ? ` (${v.reason})` : ''}
                  </li>
                ))}
                {structured.lexical.bannedPatterns.map((v) => (
                  <li key={v.value}>
                    {v.value}
                    {v.reason ? ` (${v.reason})` : ''}
                  </li>
                ))}
                {structured.endings.bannedEndings.map((v) => (
                  <li key={v}>{v}</li>
                ))}
              </ul>
            </section>
          )}
        </div>
      )}
    </section>
  )
}
