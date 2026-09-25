import { useState } from 'react'
import type { PostDraft } from '@/entities/post'
import type { VoiceRef } from '@/entities/voice'
import { activeLocale } from '@/shared/lib'
import type { ContentLanguage } from '@/shared/api'

/** What a draft is written WITH: its voice, its template and the language it is written in.
 *
 *  For a saved post the server owns all three, and each changes only through a confirmed
 *  assignment (`useAutosave.reassign`). For `/posts/new` there is no post to own them, so the
 *  screen holds the choices until the first save mints one. The create carries no 분야: it is a
 *  run option of the writing brief, whose form appears after that first save (POST-89). The
 *  language is snapshotted once when the draft opens: later interface-locale switches are
 *  presentation-only, and the explicit selector is the sole way this draft's target changes. */
export function useDraftAssignments(post: PostDraft | undefined, defaultVoiceId: string) {
  const [voice, setVoice] = useState(defaultVoiceId)
  const [template, setTemplate] = useState('')
  const [language, setLanguage] = useState<ContentLanguage>(() => activeLocale())
  return {
    voiceId: post ? post.voice.id : voice,
    setVoiceId: setVoice,
    templateId: post ? post.template.id : template,
    setTemplateId: setTemplate,
    targetLanguage: post?.targetLanguage ?? language,
    setTargetLanguage: setLanguage,
    /** The voice as the warning below the memo reads it: a draft's is a bare reference. */
    voice: (post?.voice ?? {
      id: voice,
      name: '',
      deleted: false,
      sourceLanguage: undefined,
    }) satisfies VoiceRef,
  }
}
