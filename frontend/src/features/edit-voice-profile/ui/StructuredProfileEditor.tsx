import type { VoiceAxes, VoiceProfile } from '@/entities/voice'
import { VoiceLayer } from '@/shared/api'
import { Badge } from '@/shared/ui'
import { ProfileField } from './ProfileField'

/** The six axes in their canonical order. They are listed explicitly rather than iterated from the
 *  object, because an axis the analysis never answered is ABSENT — iterating its keys would drop
 *  the unknown ones from the screen instead of reporting them. */
const AXES: ReadonlyArray<{ key: keyof VoiceAxes; label: string }> = [
  { key: 'involvement', label: '관여도' },
  { key: 'narrativity', label: '서사성' },
  { key: 'persuasionOvertness', label: '설득 노출' },
  { key: 'abstractness', label: '추상성' },
  { key: 'addresseeFocus', label: '독자 지향' },
  { key: 'humor', label: '유머' },
]

export function StructuredProfileEditor({
  ownerId,
  voiceId,
  profile,
  readOnly = false,
}: {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  readOnly?: boolean
}) {
  const structured = profile.structured
  const fields = [
    {
      label: '어휘 성격',
      layer: VoiceLayer.LEXICAL,
      field: 'description',
      value: structured.lexical.description,
    },
    {
      label: '주 종결어미',
      layer: VoiceLayer.ENDINGS,
      field: 'base_register',
      value: structured.endings.baseRegister,
    },
    {
      label: '접속 방식',
      layer: VoiceLayer.SYNTAX,
      field: 'connective_style',
      value: structured.syntax.connectiveStyle,
    },
    {
      label: '도입 방식',
      layer: VoiceLayer.STRUCTURE,
      field: 'intro_pattern',
      value: structured.structure.introPattern,
    },
    {
      label: '마무리 방식',
      layer: VoiceLayer.STRUCTURE,
      field: 'closing_pattern',
      value: structured.structure.closingPattern,
    },
  ]
  return (
    <section aria-labelledby="profile-heading">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 id="profile-heading" className="text-lg font-semibold tracking-tight">
          현재 말투 프로필
        </h2>
        <span className="text-content-tertiary text-sm">
          v{structured.version.toString()} · 완성 글 {profile.finalizedSourceCount}편
        </span>
      </div>
      {structured.empty ? (
        <p className="text-content-secondary mt-3 text-sm leading-relaxed">
          아직 배운 말투가 없어요. 첫 글도 그대로 생성할 수 있고, 완성한 글을 직접 확정하면 그때부터
          한 편씩 배웁니다.
        </p>
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
            <h3 className="font-medium">종결어미 분포</h3>
            <div className="mt-2 flex flex-wrap gap-2">
              {structured.endings.distribution.map((item) => (
                <Badge key={item.ending}>
                  {item.ending} {Math.round(item.ratio * 100)}%
                </Badge>
              ))}
            </div>
          </section>
          <section>
            <h3 className="font-medium">문장과 구조</h3>
            <dl className="text-content-secondary mt-2 grid gap-2 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-content-tertiary">평균 문장 길이</dt>
                <dd>{structured.syntax.averageSentenceChars.toFixed(1)}자</dd>
              </div>
              <div>
                <dt className="text-content-tertiary">문단당 문장</dt>
                <dd>
                  {structured.structure.paragraphSentencesMin}–
                  {structured.structure.paragraphSentencesMax}개
                </dd>
              </div>
            </dl>
          </section>
          <section>
            <h3 className="font-medium">여섯 성향 (-3~3)</h3>
            <dl className="text-content-secondary mt-2 grid grid-cols-2 gap-2 text-sm sm:grid-cols-3">
              {AXES.map((axis) => {
                const value = structured.axes[axis.key]
                return (
                  <div key={axis.key}>
                    <dt className="text-content-tertiary">{axis.label}</dt>
                    <dd className={value === undefined ? 'text-content-tertiary' : undefined}>
                      {value === undefined ? '알 수 없음' : value}
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
              <h3 className="font-medium">피할 표현</h3>
              <ul className="text-content-secondary mt-2 list-disc pl-5 text-sm">
                {structured.lexical.bannedWords.map((v) => (
                  <li key={v.value}>
                    {v.value}
                    {v.reason ? ` — ${v.reason}` : ''}
                  </li>
                ))}
                {structured.lexical.bannedPatterns.map((v) => (
                  <li key={v.value}>
                    {v.value}
                    {v.reason ? ` — ${v.reason}` : ''}
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
