import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { ModelPicker } from '@/features/configure-model-pair'
import { NextTemplateChoices } from '@/features/select-post-template'
import { DefaultVoiceChoices } from '@/features/set-default-voice'
import { DefaultGuidelineChoices } from '@/features/toggle-default-guideline'
import type { KitDefaultKey } from '../model/kit'
import { KitDropdown } from './KitDropdown'

/** Guide: a piece's name on 홈, as the dropdown that changes its default without leaving 홈 — with
 *  the very list the default is chosen from elsewhere:
 *  - 말투: the voices, the 기본 checked; a made one becomes the 기본 (VOICE-12).
 *  - 템플릿: 없음 and the templates; the one picked is what the next new post starts on.
 *  - 지침: the 기본 지침, each on or off, several in one visit (GUIDE-43).
 *  - 글 쓰는 AI: AI 토너먼트's own picker for the write stage, with its grades, its credits per post
 *    and the models that cannot be used, saved with the same SaveSelection.
 *
 *  A single pick closes the dropdown once it is saved, so the piece's fact line under it shows
 *  the new answer; a refused save keeps it open, saying so. The piece's ring is the way to its
 *  whole screen. */
export function KitDefaultControl({
  kit,
  name,
  describedBy,
  className,
}: {
  kit: KitDefaultKey
  /** The piece's name: the trigger's visible words. */
  name: string
  /** Ids of the lines that say the piece's state and its fact. */
  describedBy?: string
  className?: string
}) {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const title = t(`kit.defaults.title.${kit}`)
  return (
    <KitDropdown
      name={name}
      label={t(`kit.defaults.change.${kit}`)}
      describedBy={describedBy}
      className={className}
      // On the orbit a piece is 7rem wide: the name takes the room the padding would.
      triggerClassName="@xs:px-0.5 @md:px-2"
      title={title}
      help={t(`kit.defaults.help.${kit}`)}
    >
      {(chosen) => (
        <>
          {kit === 'voice' && (
            <DefaultVoiceChoices ownerId={ownerId} label={title} onChosen={chosen} />
          )}
          {kit === 'template' && (
            <NextTemplateChoices ownerId={ownerId} label={title} onChosen={chosen} />
          )}
          {kit === 'guideline' && (
            <DefaultGuidelineChoices ownerId={ownerId} kind="post" label={title} />
          )}
          {kit === 'ai' && <ModelPicker stage="write" onSaved={chosen} />}
        </>
      )}
    </KitDropdown>
  )
}
