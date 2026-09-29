import { useState } from 'react'
import type { PostDraft } from '@/entities/post'
import { NO_VOICE_VALUE, type Voice } from '@/entities/voice'
import { activeLocale } from '@/shared/lib'
import type { ContentLanguage } from '@/shared/api'

/** What a draft is written WITH: its voice, its template and the language it is written in.
 *
 *  For a saved post the server owns all three, and each changes only through a confirmed
 *  assignment (`useAutosave.reassign`). For `/posts/new` there is no post to own them, so the
 *  screen holds the choices until the first save mints one. The voice starts on the account's
 *  기본, or on 말투 없음 when there is none — the server never picks one (POST-101). A 기본 not yet
 *  made is no seed either: it cannot be assigned (VOICE-32), so a create carrying it would be
 *  refused. The create carries no 분야: it is a run option of the writing brief, whose form
 *  appears after that first save (POST-89). The language is snapshotted once when the draft
 *  opens: later interface-locale switches are presentation-only, and the explicit selector is
 *  the sole way this draft's target changes. */
export function useDraftAssignments(
  post: PostDraft | undefined,
  defaultVoice: Pick<Voice, 'id' | 'made'> | undefined,
) {
  const [voice, setVoice] = useState(() => (defaultVoice?.made ? defaultVoice.id : NO_VOICE_VALUE))
  const [template, setTemplate] = useState('')
  const [language, setLanguage] = useState<ContentLanguage>(() => activeLocale())
  return {
    voiceId: post ? (post.voice?.id ?? NO_VOICE_VALUE) : voice,
    setVoiceId: setVoice,
    templateId: post ? post.template.id : template,
    setTemplateId: setTemplate,
    targetLanguage: post?.targetLanguage ?? language,
    setTargetLanguage: setLanguage,
  }
}
