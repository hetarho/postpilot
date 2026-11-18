import { useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronDown } from 'lucide-react'
import { useSession } from '@/entities/session'
import { ModelPicker } from '@/features/configure-model-pair'
import { NextTemplateChoices } from '@/features/select-post-template'
import { DefaultVoiceChoices } from '@/features/set-default-voice'
import { DefaultGuidelineChoices } from '@/features/toggle-default-guideline'
import { Popover, Typography, type PopoverHandle } from '@/shared/ui'
import type { KitDefaultKey } from '../model/kit'

/** Guide: the ▾ beside a piece's name, and the dropdown it opens — anchored to the ▾ at every
 *  width, a phone's too, never a sheet. It changes the piece's default without leaving 홈, with
 *  the very list the default is chosen from elsewhere:
 *  - 말투: the voices, the 기본 checked; a made one becomes the 기본 (VOICE-12).
 *  - 템플릿: 없음 and the templates; the one picked is what the next new post starts on.
 *  - 지침: the 기본 지침, each on or off, several in one visit (GUIDE-43).
 *  - 글 쓰는 AI: AI 토너먼트's own picker for the write stage, with its grades, its credits per post
 *    and the models that cannot be used, saved with the same SaveSelection.
 *
 *  A single pick closes the dropdown once it is saved, so the piece's fact line under it shows
 *  the new answer, and focus returns to the ▾; a refused save keeps it open, saying so. The
 *  piece's name and ring stay the way to its whole screen. */
export function KitDefaultControl({ kit }: { kit: KitDefaultKey }) {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const popover = useRef<PopoverHandle>(null)
  const title = t(`kit.defaults.title.${kit}`)
  return (
    <Popover
      ref={popover}
      label={t(`kit.defaults.change.${kit}`)}
      triggerLabel={<ChevronDown aria-hidden="true" className="size-4" />}
      triggerSize="icon"
      triggerVariant="ghost"
      // A glyph-sized mark beside the name; a coarse pointer still gets the full 44px, which hangs
      // over the empty space beside the name rather than pushing the name aside.
      triggerClassName="text-content-tertiary size-7 pointer-coarse:-my-2 pointer-coarse:-ml-2 pointer-coarse:-mr-5"
      placement="below"
      align="start"
    >
      {(close) => {
        const chosen = () => {
          close()
          popover.current?.focus()
        }
        return (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-0.5">
              <Typography variant="label" as="h2" className="text-content-primary">
                {title}
              </Typography>
              <Typography variant="meta" as="p" className="text-pretty">
                {t(`kit.defaults.help.${kit}`)}
              </Typography>
            </div>
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
          </div>
        )
      }}
    </Popover>
  )
}
