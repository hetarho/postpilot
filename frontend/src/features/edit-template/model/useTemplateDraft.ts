import { useCallback, useMemo, useState } from 'react'
import {
  POST_TAG_COUNT_DEFAULT,
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
} from '@/entities/post'
import {
  TEMPLATE_PARSE_OPTIONS,
  askFields,
  canSaveTemplate,
  parseTemplate,
  type ParseFailure,
  type Template,
  type TemplateArea,
  type TemplateDraftTexts,
} from '@/entities/template'

export interface TemplateDraft {
  name: string
  description: string
  body: string
  /** The 제목 형식 (TMPL-50): '' is none, and the AI writes the title. */
  titleArea: string
  /** `undefined` is 의견 없음 — this template says nothing about that number, and assigning it
   *  leaves the post's own option alone (TMPL-47). */
  targetLength?: number
  tagCount?: number
}

/** The four texts of the draft, the part a field edits by its key. */
export type TemplateTextKey = 'name' | 'description' | 'body' | 'titleArea'

export function draftOf(stored: Template | undefined): TemplateDraft {
  return {
    name: stored?.name ?? '',
    description: stored?.description ?? '',
    body: stored?.body ?? '',
    titleArea: stored?.titleArea ?? '',
    targetLength: stored?.targetLength,
    tagCount: stored?.tagCount,
  }
}

/** One number as it is being EDITED: whether its 사용 tick is on, and the text in the field.
 *  The text outlives an unticked box on purpose — unticking and reticking must not lose what
 *  was typed (the 목표 글자 수 rule of POST-20). */
export interface NumberDraft {
  enabled: boolean
  text: string
}

function numberDraftOf(value: number | undefined): NumberDraft {
  return { enabled: value !== undefined, text: value?.toString() ?? '' }
}

/** The value this field contributes to the draft: `undefined` while unticked, and NaN while
 *  ticked with something unusable in it — which is dirty, refused by the save gate, and never
 *  confused with 의견 없음. */
function numberValue(field: NumberDraft): number | undefined {
  return field.enabled ? Number(field.text) : undefined
}

function numberValid(field: NumberDraft, min: number, max: number): boolean {
  if (!field.enabled) return true
  const parsed = Number(field.text)
  return field.text.trim() !== '' && Number.isInteger(parsed) && parsed >= min && parsed <= max
}

/** One template's draft on its screen (TMPL-25): the four texts and the two numbers as one value,
 *  what it was last saved as, and whether it can be saved at all.
 *
 *  `stored` seeds the draft once — the caller keys the screen on the template's identity, so a
 *  refetch never overwrites what is being typed — and it is the clean baseline until the user's
 *  own save lands. Saving itself is `useTemplateSave`'s: this hook only adopts what a save wrote. */
export function useTemplateDraft(stored: Template | undefined) {
  const [draft, setDraft] = useState<TemplateDraft>(() => draftOf(stored))
  // The two generation numbers are part of the same one draft (TMPL-49), kept as their own
  // editing state because a ticked field can hold text that is not yet a number.
  const [lengthField, setLengthField] = useState<NumberDraft>(() =>
    numberDraftOf(stored?.targetLength),
  )
  const [tagsField, setTagsField] = useState<NumberDraft>(() => numberDraftOf(stored?.tagCount))
  const lengthValid = numberValid(lengthField, POST_TARGET_LENGTH_MIN, POST_TARGET_LENGTH_MAX)
  const tagsValid = numberValid(tagsField, POST_TAG_COUNT_MIN, POST_TAG_COUNT_MAX)
  // What the last successful save wrote, taken from the mutation's OWN response. The directory
  // query lags a save by a refetch, so comparing against it alone would leave the screen dirty
  // for that whole window — which re-enables 저장 and makes the leave guard warn about a
  // template that was just saved.
  const [savedBaseline, setSavedBaseline] = useState<TemplateDraft | null>(null)
  const [saved, setSaved] = useState(false)
  // The template request (TMPL-58): its answer and its undo replace the draft's four texts, never
  // the two numbers (TMPL-60), and make the draft dirty like any edit (TMPL-63). Stable, because
  // the request applies it from an effect.
  const applyRequest = useCallback((next: TemplateDraftTexts) => {
    setDraft((current) => ({
      ...current,
      name: next.name,
      description: next.description,
      titleArea: next.titleArea,
      body: next.body,
    }))
    setSaved(false)
  }, [])

  // The name and the description are user prose and are trimmed. The BODY is not: it is the
  // canonical serialization of the composition, and trimming it would make a stored body with
  // significant outer bytes read as dirty on open and be rewritten on save — which is exactly
  // what TMPL-8 forbids.
  const trimmed: TemplateDraft = {
    name: draft.name.trim(),
    description: draft.description.trim(),
    body: draft.body,
    // Untrimmed, for the body's reason.
    titleArea: draft.titleArea,
    targetLength: numberValue(lengthField),
    tagCount: numberValue(tagsField),
  }
  // `stored` is still read here, so a refetch that lands after a save changes nothing:
  // `savedBaseline` already describes the saved state.
  const baseline = savedBaseline ?? draftOf(stored)
  const dirty =
    trimmed.name !== baseline.name ||
    trimmed.description !== baseline.description ||
    trimmed.body !== baseline.body ||
    trimmed.titleArea !== baseline.titleArea ||
    trimmed.targetLength !== baseline.targetLength ||
    trimmed.tagCount !== baseline.tagCount
  // Parsed ONCE, here, as the one document the two areas are (TMPL-50): the save gate and the
  // error each area shows are the same answer, so they cannot disagree. The builder emits only
  // text that parses, so this changes nothing for a builder-only flow — it is what makes "a
  // template that does not parse cannot be saved from EITHER mode" true (TMPL-30,
  // TMPL-7), and what catches a failure only the two areas together have.
  const parsed = parseTemplate(trimmed.titleArea, trimmed.body, TEMPLATE_PARSE_OPTIONS)
  const failureIn = (area: TemplateArea): ParseFailure | null =>
    !parsed.ok && parsed.failure.area === area ? parsed.failure : null
  // Two rows asking under one title, in either area. A composition leaves such a row OUT of its
  // text, so the draft parses and nothing here would otherwise notice — and saving would
  // silently drop the row the author is looking at (TMPL-44). One flag per area, each setter
  // handed over as is: a stable reference, so neither composition re-reports on every render.
  const [titleAskConflict, setTitleAskConflict] = useState(false)
  const [bodyAskConflict, setBodyAskConflict] = useState(false)
  // The title's data fields come first in the one namespace (TMPL-55), so a body row asking
  // under one of them is the row that yields.
  const titleAskLabels = useMemo(
    () =>
      new Set(
        askFields(draft.titleArea, { ...TEMPLATE_PARSE_OPTIONS, titleArea: true }).map(
          (field) => field.label,
        ),
      ),
    [draft.titleArea],
  )
  // Everything about the draft itself that can refuse 저장. Whether there is anything to save
  // (`dirty`) and whether something is running are the save's own conditions.
  const valid =
    canSaveTemplate(trimmed) &&
    lengthValid &&
    tagsValid &&
    parsed.ok &&
    !titleAskConflict &&
    !bodyAskConflict

  // The clean baseline is what the server stored, from the mutation's own response (TMPL-25),
  // and the draft's text takes it too: the server trims the body at its edges (TMPL-6), so a
  // saved screen is clean and shows exactly what was written. The fields are disabled while the
  // save runs, so nothing typed meanwhile is overwritten. A response without a template leaves
  // the sent draft as the baseline.
  const adoptSaved = (template: Template | undefined) => {
    if (!template) {
      setSavedBaseline(trimmed)
      return
    }
    const written = draftOf(template)
    setSavedBaseline(written)
    setDraft((current) => ({
      ...current,
      name: written.name,
      description: written.description,
      body: written.body,
      titleArea: written.titleArea,
    }))
  }

  const setText = (key: TemplateTextKey) => (value: string) => {
    setDraft((current) => ({ ...current, [key]: value }))
    setSaved(false)
  }

  // Ticking reveals a field with a usable number ALREADY in it, and a value typed earlier in
  // this session outranks the default — the same rule the post's own 목표 글자 수 follows, so
  // the field behaves identically in the two places it is met (POST-20).
  const numberField =
    (set: typeof setLengthField, fallback: number) => (next: Partial<NumberDraft>) => {
      set((current) => {
        const merged = { ...current, ...next }
        if (next.enabled && !current.text) merged.text = String(fallback)
        return merged
      })
      setSaved(false)
    }

  return {
    draft,
    /** What a save sends: the prose trimmed, the composition untouched, the numbers resolved. */
    trimmed,
    dirty,
    valid,
    /** The last save landed and nothing was edited since. */
    saved,
    lengthField,
    tagsField,
    lengthValid,
    tagsValid,
    failureIn,
    titleAskLabels,
    onTitleAskConflict: setTitleAskConflict,
    onBodyAskConflict: setBodyAskConflict,
    setText,
    setTargetLength: numberField(setLengthField, POST_TARGET_LENGTH_DEFAULT),
    setTagCount: numberField(setTagsField, POST_TAG_COUNT_DEFAULT),
    applyRequest,
    adoptSaved,
    markSaved: () => setSaved(true),
  }
}

export type TemplateDraftState = ReturnType<typeof useTemplateDraft>
