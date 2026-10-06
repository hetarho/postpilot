import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { twMerge } from 'tailwind-merge'
import { useSession } from '@/entities/session'
import { AIAuthoringStudio } from '@/widgets/ai-authoring-studio'
import {
  TEMPLATE_LIMITS,
  TEMPLATE_MAX_PER_ACCOUNT,
  TemplateComposition,
  TemplatePreview,
  TemplateSource,
  remainingChars,
  useTemplates,
  type Template,
  type TemplateArea,
} from '@/entities/template'
import { useTemplateDraft, useTemplateSave, type NumberDraft } from '@/features/edit-template'
import {
  TemplateRequestBox,
  useTemplateRequest,
  type TemplateRequestSample,
} from '@/features/request-template'
import {
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
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
 *  editor from `entities/template`, the draft and its one save from `features/edit-template`, and
 *  the AI request from `features/request-template`. */
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
    <TemplateAuthoringPage
      key={stored?.id ?? 'new'}
      ownerId={ownerId}
      stored={stored}
      sampleSlug={stored ? undefined : from}
      templateCount={templates.length}
      onPublished={refetch}
    />
  )
}

function TemplateAuthoringPage({
  ownerId,
  stored,
  sampleSlug,
  templateCount,
  onPublished,
}: {
  ownerId: string
  stored: Template | undefined
  sampleSlug?: string
  templateCount: number
  onPublished: () => void
}) {
  const { t } = useTranslation('authoring')
  const { t: templateText } = useTranslation('templates')
  const navigate = useNavigate()
  const [manual, setManual] = useState(Boolean(sampleSlug))
  const [busy, setBusy] = useState(false)
  if (manual)
    return (
      <Editor
        ownerId={ownerId}
        stored={stored}
        sampleSlug={sampleSlug}
        templateCount={templateCount}
      />
    )
  return (
    <main className={pageStyles({ width: 'board', className: 'flex flex-1 flex-col py-8' })}>
      <BackLink />
      <Typography variant="title" as="h1" className="text-content-secondary mt-6">
        {stored?.name ?? templateText('page.new')}
      </Typography>
      <AIAuthoringStudio
        ownerId={ownerId}
        kind="post-template"
        targetId={stored?.id}
        onBusyChange={setBusy}
        onSaved={(saved) => {
          onPublished()
          void navigate({
            to: '/templates/$templateId',
            params: { templateId: saved.id },
            replace: true,
          })
        }}
        className="mt-8"
      />
      <Button
        variant="ghost"
        className="mt-8 self-start"
        disabled={busy}
        onClick={() => setManual(true)}
      >
        {t('host.manual')}
      </Button>
    </main>
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
  // The one draft, the request that may rewrite its texts, and the one save over both: the draft
  // comes first because the request's answer lands in it, and the save reads the two together.
  const draftState = useTemplateDraft(stored)
  const { draft, failureIn } = draftState
  const request = useTemplateRequest(draftState.applyRequest)
  const saving = useTemplateSave({ ownerId, stored, draft: draftState, request })
  const { locked } = saving
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

  // `stored` is still read for the heading and for the create-vs-update decision, so a refetch
  // that lands after a save changes neither: the draft's baseline already describes the saved
  // state.

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
          <NameField value={draft.name} onChange={draftState.setText('name')} disabled={locked} />
          <DescriptionField
            value={draft.description}
            onChange={draftState.setText('description')}
            disabled={locked}
          />
          <NumberField
            id="template-target-length"
            tick={t('numbers.targetLengthTick', { ns: 'templates' })}
            label={t('numbers.targetLength', { ns: 'templates' })}
            help={t('numbers.targetLengthHelp', { ns: 'templates' })}
            field={draftState.lengthField}
            min={POST_TARGET_LENGTH_MIN}
            max={POST_TARGET_LENGTH_MAX}
            valid={draftState.lengthValid}
            disabled={locked}
            onChange={draftState.setTargetLength}
          />
          <NumberField
            id="template-tag-count"
            tick={t('numbers.tagCountTick', { ns: 'templates' })}
            label={t('numbers.tagCount', { ns: 'templates' })}
            help={t('numbers.tagCountHelp', { ns: 'templates' })}
            field={draftState.tagsField}
            min={POST_TAG_COUNT_MIN}
            max={POST_TAG_COUNT_MAX}
            valid={draftState.tagsValid}
            disabled={locked}
            onChange={draftState.setTagCount}
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
                    onChange={draftState.setText('titleArea')}
                    disabled={locked}
                    failure={failureIn('title_area')}
                    autoFocus={focusSource === 'title_area'}
                    className="mt-3"
                  />
                ) : (
                  <TemplateComposition
                    area="title_area"
                    onAskConflict={draftState.onTitleAskConflict}
                    value={draft.titleArea}
                    onChange={draftState.setText('titleArea')}
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
                    onChange={draftState.setText('body')}
                    disabled={locked}
                    failure={failureIn('body')}
                    autoFocus={focusSource === 'body'}
                    className="mt-3"
                  />
                ) : (
                  <TemplateComposition
                    onAskConflict={draftState.onBodyAskConflict}
                    value={draft.body}
                    onChange={draftState.setText('body')}
                    disabled={locked}
                    takenAskTitles={draftState.titleAskLabels}
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
      {saving.failed && <FieldMessage className="mt-auto pt-6">{saving.errorMessage}</FieldMessage>}
      {draftState.saved && !saving.failed && (
        <Typography variant="meta" as="p" role="status" className="mt-auto pt-6">
          {t('screen.saved', { ns: 'templates' })}
        </Typography>
      )}

      {/* Docked at every width, not only on the phone: the distance between the composition and
          the control that commits it is there on a desk too (THEME-24). */}
      <ActionBar
        ariaLabel={t('screen.saveDockAria', { ns: 'templates' })}
        className={saving.failed || draftState.saved ? undefined : 'mt-auto'}
      >
        <Button
          variant="cta"
          disabled={saving.blocked}
          pending={saving.pending}
          onClick={() => void saving.save()}
          className="w-full"
        >
          {t('action.save', { ns: 'common' })}
        </Button>
      </ActionBar>

      <Dialog
        open={saving.leave.asking}
        title={t(request.running ? 'request.leaveTitle' : 'screen.leaveTitle', { ns: 'templates' })}
        confirmLabel={t(request.running ? 'request.leaveConfirm' : 'screen.leaveConfirm', {
          ns: 'templates',
        })}
        onClose={saving.leave.stay}
        onConfirm={saving.leave.go}
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
