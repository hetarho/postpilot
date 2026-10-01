import { useCallback, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useBlocker, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { twMerge } from 'tailwind-merge'
import { useSession } from '@/entities/session'
import {
  TEMPLATE_LIMITS,
  TEMPLATE_MAX_PER_ACCOUNT,
  TEMPLATE_PARSE_OPTIONS,
  TemplateComposition,
  TemplatePreview,
  TemplateSource,
  askFields,
  canSaveTemplate,
  parseTemplate,
  remainingChars,
  useCreateTemplate,
  useTemplates,
  useUpdateTemplate,
  type Template,
  type TemplateArea,
  type TemplateDraftTexts,
} from '@/entities/template'
import {
  TemplateRequestBox,
  useTemplateRequest,
  type TemplateRequestSample,
} from '@/features/request-template'
import {
  POST_TAG_COUNT_DEFAULT,
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_DEFAULT,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
  usePost,
} from '@/entities/post'
import {
  ActionBar,
  Button,
  Checkbox,
  Dialog,
  FieldCount,
  FieldLabel,
  FieldMessage,
  Notice,
  SegmentedControl,
  TextField,
  Textarea,
  Typography,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** One template's own screen — the composition of `/templates/$templateId` and `/templates/new`.
 *
 *  ONE DRAFT, ONE SAVE. The three fields used to save independently the moment each was confirmed,
 *  which is why a template had no screen at all: every row had to carry a whole editor. Here they
 *  are one value that leaves together, so the composition can be rearranged for as long as it
 *  takes without half of it reaching the server (TMPL-25).
 *
 *  Composition only, like every other page: the fields come from `shared/ui`, the composition
 *  editor from `entities/template`, and the two writes from the features that own them. */
export function TemplatePage() {
  // `strict: false` because ONE component serves both routes: `/templates/new` has no param, and
  // asking for it strictly there would throw rather than mean "a template that does not exist
  // yet". Creating and editing are the same screen (TMPL-25), so they are the same component.
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
  // `/templates/new?from=<slug>`: a post attached as the request's sample (TMPL-64).
  const { from } = useSearch({ strict: false }) as { from?: string }
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const stored = templateId ? templates.find((template) => template.id === templateId) : undefined

  if (isError) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          <span>{t('loadFailed', { ns: 'templates' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      </main>
    )
  }
  if (templateId && (isPending || (!stored && isFetching))) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      </main>
    )
  }
  // `!isFetching` matters right after a create: the screen replaces to the new id while the
  // directory invalidation is still in flight, so the row is legitimately not there yet.
  if (templateId && !stored && !isFetching) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          {t('screen.notFound', { ns: 'templates' })}
        </Notice>
      </main>
    )
  }

  // Keyed on the template's identity so the draft is seeded once per subject: switching templates
  // must not carry the previous one's unsaved edits, and a refetch must not overwrite what is
  // being typed. `stored` is the baseline the dirty check compares against, and it only moves
  // when the user's own save lands.
  return (
    <Editor
      key={stored?.id ?? 'new'}
      ownerId={ownerId}
      stored={stored}
      sampleSlug={stored ? undefined : from}
      templateCount={templates.length}
    />
  )
}

function BackLink() {
  const { t } = useTranslation('templates')
  return (
    <Link
      to="/templates"
      className={typographyStyles({
        variant: 'label',
        className:
          'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center underline',
      })}
    >
      {t('screen.backToList')}
    </Link>
  )
}

const COMPOSITION_PANEL_ID = 'template-composition-panel'
const EDIT_PANEL_ID = 'template-edit-panel'
const PREVIEW_PANEL_ID = 'template-preview-panel'

interface Draft {
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

function draftOf(stored: Template | undefined): Draft {
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
interface NumberDraft {
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

function Editor({
  ownerId,
  stored,
  sampleSlug: initialSample,
  templateCount,
}: {
  ownerId: string
  stored: Template | undefined
  sampleSlug?: string
  templateCount: number
}) {
  const { t } = useTranslation(['templates', 'common'])
  const navigate = useNavigate()
  const [draft, setDraft] = useState<Draft>(() => draftOf(stored))
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
  const [savedBaseline, setSavedBaseline] = useState<Draft | null>(null)
  const [saved, setSaved] = useState(false)
  // Which way the composition is being edited. Two renderings of ONE field, never both at once:
  // the builder reseeds its rows from the body on mount, which is the same "value from outside"
  // path a refetch takes (TMPL-29), so switching needs no synchronisation of its own.
  const [mode, setMode] = useState<'builder' | 'source'>('builder')
  // Set only by 원문에서 고치기, to the area that failed: arriving there by the user's own choice
  // of the tab should not steal the caret, but arriving there to fix a parse error should put it
  // in the text that has the error.
  const [focusSource, setFocusSource] = useState<TemplateArea | null>(null)
  // Below lg the composition and the preview take turns; at lg both stand side by side and this
  // switch is hidden (TMPL-67). Page-local: it never touches the draft or the leave guard.
  const [view, setView] = useState<'compose' | 'preview'>('compose')
  const create = useCreateTemplate(ownerId)
  const update = useUpdateTemplate(ownerId, stored?.id ?? '')
  // The template request (TMPL-58): its answer and its undo replace the draft's four texts, never
  // the two numbers (TMPL-60), and make the draft dirty like any edit (TMPL-63).
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
  const request = useTemplateRequest(applyRequest)
  // The post a new template follows (TMPL-64), until its chip is removed.
  const [sampleSlug, setSampleSlug] = useState(initialSample ?? '')
  const samplePost = usePost(sampleSlug, { enabled: sampleSlug !== '' })
  const sample: TemplateRequestSample | undefined =
    sampleSlug === ''
      ? undefined
      : samplePost.failure
        ? { kind: 'missing' }
        : samplePost.post
          ? {
              kind: 'post',
              slug: sampleSlug,
              title: samplePost.post.content?.title || samplePost.post.title,
            }
          : undefined

  // The name and the description are user prose and are trimmed. The BODY is not: it is the
  // canonical serialization of the composition, and trimming it would make a stored body with
  // significant outer bytes read as dirty on open and be rewritten on save — which is exactly
  // what TMPL-8 forbids.
  const trimmed: Draft = {
    name: draft.name.trim(),
    description: draft.description.trim(),
    body: draft.body,
    // Untrimmed, for the body's reason.
    titleArea: draft.titleArea,
    targetLength: numberValue(lengthField),
    tagCount: numberValue(tagsField),
  }
  const baseline = savedBaseline ?? draftOf(stored)
  const dirty =
    trimmed.name !== baseline.name ||
    trimmed.description !== baseline.description ||
    trimmed.body !== baseline.body ||
    trimmed.titleArea !== baseline.titleArea ||
    trimmed.targetLength !== baseline.targetLength ||
    trimmed.tagCount !== baseline.tagCount
  const pending = create.isPending || update.isPending
  // A running request locks the draft (TMPL-63): its answer lands in it.
  const locked = pending || request.running
  const errorMessage = create.errorMessage || update.errorMessage
  const failed = create.isError || update.isError
  // Parsed ONCE, here, as the one document the two areas are (TMPL-50): the save gate and the
  // error each area shows are the same answer, so they cannot disagree. The builder emits only
  // text that parses, so this changes nothing for a builder-only flow — it is what makes "a
  // template that does not parse cannot be saved from EITHER mode" true (TMPL-30,
  // TMPL-7), and what catches a failure only the two areas together have.
  const parsed = parseTemplate(trimmed.titleArea, trimmed.body, TEMPLATE_PARSE_OPTIONS)
  const failureIn = (area: TemplateArea) =>
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
  const blocked =
    !dirty ||
    !canSaveTemplate(trimmed) ||
    !lengthValid ||
    !tagsValid ||
    !parsed.ok ||
    titleAskConflict ||
    bodyAskConflict ||
    locked

  // A REF, not state: the post-save redirect below runs in the same tick as the state update
  // that would clear `dirty`, and the blocker reads its render-time closure — so without this the
  // screen would intercept its own navigation and ask whether to discard a template it had just
  // created. It needs no reset: the route change remounts this component.
  const leavingAfterSave = useRef(false)
  // A running request is something to lose as much as an unsaved draft: leaving cancels it.
  const guard = () => request.running || (!leavingAfterSave.current && dirty && !pending)

  // `enableBeforeUnload` is a FUNCTION, not the default `true`: the beforeunload path does not
  // consult `shouldBlockFn`, so leaving it alone would make the browser prompt on every reload of
  // a clean screen. A tab close still warns when there is something to lose — the browser's own
  // untranslatable prompt is a poor message, but losing an unsaved composition silently is worse.
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

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

  const save = async () => {
    if (blocked) return
    try {
      if (stored) {
        // All three fields in one call. They are one decision now, and the server applies a
        // present field and leaves an absent one alone — so sending three is one transaction,
        // not a read-modify-write of anything the user did not touch on this screen.
        const updated = await update.saveAll(trimmed)
        adoptSaved(updated.template)
        setSaved(true)
        return
      }
      const created = await create.create(trimmed)
      const id = created.template?.id
      // The baseline moves BEFORE the navigation, or the blocker below would intercept the
      // screen's own redirect and ask whether to discard a template that was just created.
      adoptSaved(created.template)
      if (!id) {
        // A create that answered without an id has nothing to navigate to. Staying put with the
        // draft intact is the honest outcome; navigating to an empty param would 404.
        setSaved(true)
        return
      }
      leavingAfterSave.current = true
      // `replace`, so Back from the saved template goes to the list rather than to a `new`
      // screen that no longer describes anything.
      await navigate({ to: '/templates/$templateId', params: { templateId: id }, replace: true })
    } catch {
      // The mutation's message renders above the dock.
    }
  }

  const field = (key: 'name' | 'description' | 'body' | 'titleArea') => (value: string) => {
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

  // `stored` is still read for the heading and for the create-vs-update decision, so a refetch
  // that lands after a save changes neither: `savedBaseline` already describes the saved state.

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col lg:max-w-7xl' })}>
      <BackLink />
      <Typography variant="display" className="mt-2 block">
        {stored ? stored.name : t('screen.newTitle', { ns: 'templates' })}
      </Typography>

      {/* Above both columns: open as the first action on an empty new template, a collapsed
          AI에게 요청 once the draft holds anything (TMPL-62). */}
      <TemplateRequestBox
        request={request}
        draft={{
          name: draft.name,
          description: draft.description,
          titleArea: draft.titleArea,
          body: draft.body,
        }}
        templateId={stored?.id}
        startOpen={
          !stored &&
          (sampleSlug !== '' ||
            (draft.name === '' &&
              draft.description === '' &&
              draft.titleArea === '' &&
              draft.body === ''))
        }
        atCap={!stored && templateCount >= TEMPLATE_MAX_PER_ACCOUNT}
        sample={sample}
        onRemoveSample={() => setSampleSlug('')}
        className="mt-6"
      />

      <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:items-start lg:gap-12">
        <div className="min-w-0">
          <NameField value={draft.name} onChange={field('name')} disabled={locked} />
          <DescriptionField
            value={draft.description}
            onChange={field('description')}
            disabled={locked}
          />
          <NumberField
            id="template-target-length"
            tick={t('numbers.targetLengthTick', { ns: 'templates' })}
            label={t('numbers.targetLength', { ns: 'templates' })}
            help={t('numbers.targetLengthHelp', { ns: 'templates' })}
            field={lengthField}
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            valid={lengthValid}
            disabled={locked}
            onChange={numberField(setLengthField, POST_TARGET_LENGTH_DEFAULT)}
          />
          <NumberField
            id="template-tag-count"
            tick={t('numbers.tagCountTick', { ns: 'templates' })}
            label={t('numbers.tagCount', { ns: 'templates' })}
            help={t('numbers.tagCountHelp', { ns: 'templates' })}
            field={tagsField}
            min={POST_TAG_COUNT_MIN}
            max={POST_TAG_COUNT_MAX}
            valid={tagsValid}
            disabled={locked}
            onChange={numberField(setTagsField, POST_TAG_COUNT_DEFAULT)}
          />

          {/* The phone's one-at-a-time switch. It stands above the composition area and below the
          fields, which stay where they are (TMPL-67). */}
          <SegmentedControl
            value={view}
            options={[
              { value: 'compose', label: t('screen.view.compose', { ns: 'templates' }) },
              { value: 'preview', label: t('screen.view.preview', { ns: 'templates' }) },
            ]}
            onChange={setView}
            ariaLabel={t('screen.view.aria', { ns: 'templates' })}
            controls={view === 'compose' ? EDIT_PANEL_ID : PREVIEW_PANEL_ID}
            className="mt-8 lg:hidden"
          />
          <div
            id={EDIT_PANEL_ID}
            role="tabpanel"
            className={view === 'preview' ? 'hidden lg:block' : undefined}
          >
            {/* ONE switch for both texts: they are two parts of one template, edited the same way. */}
            <SegmentedControl
              value={mode}
              options={[
                { value: 'builder', label: t('screen.mode.builder', { ns: 'templates' }) },
                { value: 'source', label: t('screen.mode.source', { ns: 'templates' }) },
              ]}
              onChange={(next) => {
                setMode(next)
                // Only the fix button asks for the caret; picking the tab does not.
                if (next === 'builder') setFocusSource(null)
              }}
              ariaLabel={t('screen.mode.aria', { ns: 'templates' })}
              controls={COMPOSITION_PANEL_ID}
              disabled={locked}
              className="mt-8"
            />
            <div id={COMPOSITION_PANEL_ID} role="tabpanel">
              {/* The title comes first because it reads first — in the post and in the one data-field
            namespace (TMPL-50, TMPL-55). */}
              <section aria-labelledby="template-title-area-heading" className="mt-6">
                <Typography variant="title" id="template-title-area-heading">
                  {t('screen.titleArea.heading', { ns: 'templates' })}{' '}
                  {t('form.optional', { ns: 'common' })}
                </Typography>
                <Typography
                  variant="body"
                  as="p"
                  className="text-content-secondary max-w-measure mt-1"
                >
                  {t('screen.titleArea.help', { ns: 'templates' })}
                </Typography>
                {mode === 'source' ? (
                  <TemplateSource
                    area="title_area"
                    value={draft.titleArea}
                    onChange={field('titleArea')}
                    disabled={locked}
                    failure={failureIn('title_area')}
                    autoFocus={focusSource === 'title_area'}
                    className="mt-3"
                  />
                ) : (
                  <TemplateComposition
                    area="title_area"
                    onAskConflict={setTitleAskConflict}
                    value={draft.titleArea}
                    onChange={field('titleArea')}
                    disabled={locked}
                    failure={failureIn('title_area')}
                    onFixInSource={() => {
                      setFocusSource('title_area')
                      setMode('source')
                    }}
                    className="mt-3"
                  />
                )}
              </section>

              <section aria-labelledby="template-composition-heading" className="mt-8">
                <Typography variant="title" id="template-composition-heading">
                  {t('create.body', { ns: 'templates' })}
                </Typography>
                <Typography
                  variant="body"
                  as="p"
                  className="text-content-secondary max-w-measure mt-1"
                >
                  {t(mode === 'source' ? 'screen.sourceHelp' : 'screen.compositionHelp', {
                    ns: 'templates',
                  })}
                </Typography>
                {mode === 'source' ? (
                  <TemplateSource
                    value={draft.body}
                    onChange={field('body')}
                    disabled={locked}
                    failure={failureIn('body')}
                    autoFocus={focusSource === 'body'}
                    className="mt-3"
                  />
                ) : (
                  <TemplateComposition
                    onAskConflict={setBodyAskConflict}
                    value={draft.body}
                    onChange={field('body')}
                    disabled={locked}
                    takenAskTitles={titleAskLabels}
                    failure={failureIn('body')}
                    onFixInSource={() => {
                      setFocusSource('body')
                      setMode('source')
                    }}
                    className="mt-3"
                  />
                )}
              </section>
            </div>
          </div>
        </div>

        {/* Beside the composition at lg, kept in view while it scrolls; one tap away below. */}
        <aside
          id={PREVIEW_PANEL_ID}
          className={twMerge(
            'lg:top-header mt-8 lg:sticky lg:mt-6 lg:max-h-[calc(100dvh-var(--spacing-header)-1.5rem)] lg:overflow-y-auto',
            view === 'compose' && 'hidden lg:block',
          )}
        >
          <TemplatePreview titleArea={draft.titleArea} body={draft.body} />
        </aside>
      </div>

      {/* The state this screen has to report goes in one place, above the control that produced
          it, so a refusal is read where the thumb already is (THEME-24). */}
      {failed && <FieldMessage className="mt-auto pt-6">{errorMessage}</FieldMessage>}
      {saved && !failed && (
        <Typography variant="meta" as="p" role="status" className="mt-auto pt-6">
          {t('screen.saved', { ns: 'templates' })}
        </Typography>
      )}

      {/* Docked at every width, not only on the phone: the distance between the composition and
          the control that commits it is there on a desk too (THEME-24). */}
      <ActionBar
        ariaLabel={t('screen.saveDockAria', { ns: 'templates' })}
        className={failed || saved ? undefined : 'mt-auto'}
      >
        <Button
          variant="cta"
          disabled={blocked}
          pending={pending}
          onClick={() => void save()}
          className="w-full"
        >
          {t('action.save', { ns: 'common' })}
        </Button>
      </ActionBar>

      <Dialog
        open={blocker.status === 'blocked'}
        title={t(request.running ? 'request.leaveTitle' : 'screen.leaveTitle', { ns: 'templates' })}
        confirmLabel={t(request.running ? 'request.leaveConfirm' : 'screen.leaveConfirm', {
          ns: 'templates',
        })}
        onClose={() => blocker.reset?.()}
        onConfirm={() => {
          // Leaving cancels the running request; it is not awaited (TMPL-63).
          if (request.running) request.cancel()
          blocker.proceed?.()
        }}
      >
        {t(request.running ? 'request.leaveDescription' : 'screen.leaveDescription', {
          ns: 'templates',
        })}
      </Dialog>
    </main>
  )
}

function NameField({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (value: string) => void
  disabled: boolean
}) {
  const { t } = useTranslation(['templates', 'common'])
  const left = remainingChars(value, TEMPLATE_LIMITS.name)
  return (
    <div className="mt-6">
      <FieldLabel htmlFor="template-name">{t('page.name', { ns: 'templates' })}</FieldLabel>
      <TextField
        id="template-name"
        value={value}
        disabled={disabled}
        autoComplete="off"
        placeholder={t('create.namePlaceholder', { ns: 'templates' })}
        aria-invalid={left < 0 || undefined}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1"
      />
      <FieldCount left={left} />
    </div>
  )
}

/** One of the template's two generation numbers: a 사용 tick and, when it is on, a number field
 *  (TMPL-49). Unticked is 의견 없음 — assigning this template then leaves the post's own
 *  option alone, which is why the tick is not a "0" and cannot be one. */
function NumberField({
  id,
  tick,
  label,
  help,
  field,
  min,
  max,
  valid,
  disabled,
  onChange,
}: {
  id: string
  /** The 사용 question. The field below carries the plain name, so a screen reader never hears
   *  the same words twice for two different controls. */
  tick: string
  label: string
  help: string
  field: NumberDraft
  min: number
  max: number
  valid: boolean
  disabled: boolean
  onChange: (next: Partial<NumberDraft>) => void
}) {
  const { t } = useTranslation(['templates', 'common'])
  return (
    <div className="mt-4">
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={field.enabled}
          disabled={disabled}
          onChange={(event) => onChange({ enabled: event.target.checked })}
        />
        {tick}
      </label>
      <Typography variant="meta" as="p" className="text-content-secondary max-w-measure">
        {help}
      </Typography>
      {field.enabled && (
        <div className="mt-2">
          <FieldLabel htmlFor={id}>{label}</FieldLabel>
          <TextField
            id={id}
            type="number"
            min={min}
            max={max}
            value={field.text}
            disabled={disabled}
            autoComplete="off"
            aria-invalid={!valid || undefined}
            onChange={(event) => onChange({ text: event.target.value })}
            className="mt-1"
          />
          {!valid && (
            <FieldMessage className="mt-1">
              {t('numbers.range', { ns: 'templates', min, max })}
            </FieldMessage>
          )}
        </div>
      )}
    </div>
  )
}

function DescriptionField({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (value: string) => void
  disabled: boolean
}) {
  const { t } = useTranslation(['templates', 'common'])
  const left = remainingChars(value, TEMPLATE_LIMITS.description)
  return (
    <div className="mt-4">
      <FieldLabel htmlFor="template-description">
        {t('create.description', { ns: 'templates' })} {t('form.optional', { ns: 'common' })}
      </FieldLabel>
      <Textarea
        id="template-description"
        value={value}
        disabled={disabled}
        rows={2}
        autoGrow
        placeholder={t('create.descriptionPlaceholder', { ns: 'templates' })}
        aria-invalid={left < 0 || undefined}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1"
      />
      <FieldCount left={left} />
    </div>
  )
}
