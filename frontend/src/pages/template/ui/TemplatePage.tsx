import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useBlocker, useNavigate, useParams } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import {
  TEMPLATE_LIMITS,
  TemplateComposition,
  canSaveTemplate,
  remainingChars,
  useTemplates,
  type Template,
} from '@/entities/template'
import { useCreateTemplate } from '@/features/create-template'
import { useUpdateTemplate } from '@/features/edit-template'
import {
  ActionBar,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  Notice,
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
 *  takes without half of it reaching the server (change 30).
 *
 *  Composition only, like every other page: the fields come from `shared/ui`, the composition
 *  editor from `entities/template`, and the two writes from the features that own them. */
export function TemplatePage() {
  // `strict: false` because ONE component serves both routes: `/templates/new` has no param, and
  // asking for it strictly there would throw rather than mean "a template that does not exist
  // yet". Creating and editing are the same screen (change 30), so they are the same component.
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
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
  return <Editor key={stored?.id ?? 'new'} ownerId={ownerId} stored={stored} />
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

interface Draft {
  name: string
  description: string
  body: string
}

function draftOf(stored: Template | undefined): Draft {
  return {
    name: stored?.name ?? '',
    description: stored?.description ?? '',
    body: stored?.body ?? '',
  }
}

function Editor({ ownerId, stored }: { ownerId: string; stored: Template | undefined }) {
  const { t } = useTranslation(['templates', 'common'])
  const navigate = useNavigate()
  const [draft, setDraft] = useState<Draft>(() => draftOf(stored))
  // What the last successful save wrote, taken from the mutation's OWN response. The directory
  // query lags a save by a refetch, so comparing against it alone would leave the screen dirty
  // for that whole window — which re-enables 저장 and makes the leave guard warn about a
  // template that was just saved.
  const [savedBaseline, setSavedBaseline] = useState<Draft | null>(null)
  const [saved, setSaved] = useState(false)
  const create = useCreateTemplate(ownerId)
  const update = useUpdateTemplate(ownerId, stored?.id ?? '')

  // The name and the description are user prose and are trimmed. The BODY is not: it is the
  // canonical serialization of the composition, and trimming it would make a stored body with
  // significant outer bytes read as dirty on open and be rewritten on save — which is exactly
  // the byte-identity change 30 A11 forbids.
  const trimmed: Draft = {
    name: draft.name.trim(),
    description: draft.description.trim(),
    body: draft.body,
  }
  const baseline = savedBaseline ?? draftOf(stored)
  const dirty =
    trimmed.name !== baseline.name ||
    trimmed.description !== baseline.description ||
    trimmed.body !== baseline.body
  const pending = create.isPending || update.isPending
  const errorMessage = create.errorMessage || update.errorMessage
  const failed = create.isError || update.isError
  const blocked = !dirty || !canSaveTemplate(trimmed) || pending

  // A REF, not state: the post-save redirect below runs in the same tick as the state update
  // that would clear `dirty`, and the blocker reads its render-time closure — so without this the
  // screen would intercept its own navigation and ask whether to discard a template it had just
  // created. It needs no reset: the route change remounts this component.
  const leavingAfterSave = useRef(false)
  const guard = () => !leavingAfterSave.current && dirty && !pending

  // `enableBeforeUnload` is a FUNCTION, not the default `true`: the beforeunload path does not
  // consult `shouldBlockFn`, so leaving it alone would make the browser prompt on every reload of
  // a clean screen. A tab close still warns when there is something to lose — the browser's own
  // untranslatable prompt is a poor message, but losing an unsaved composition silently is worse.
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })

  const save = async () => {
    if (blocked) return
    try {
      if (stored) {
        // All three fields in one call. They are one decision now, and the server applies a
        // present field and leaves an absent one alone — so sending three is one transaction,
        // not a read-modify-write of anything the user did not touch on this screen.
        await update.saveAll(trimmed)
        setSavedBaseline(trimmed)
        setSaved(true)
        return
      }
      const created = await create.create(trimmed)
      const id = created.template?.id
      // The baseline moves BEFORE the navigation, or the blocker below would intercept the
      // screen's own redirect and ask whether to discard a template that was just created.
      setSavedBaseline(trimmed)
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

  const field = (key: keyof Draft) => (value: string) => {
    setDraft((current) => ({ ...current, [key]: value }))
    setSaved(false)
  }

  // `stored` is still read for the heading and for the create-vs-update decision, so a refetch
  // that lands after a save changes neither: `savedBaseline` already describes the saved state.

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <BackLink />
      <Typography variant="display" className="mt-2 block">
        {stored ? stored.name : t('screen.newTitle', { ns: 'templates' })}
      </Typography>

      <NameField value={draft.name} onChange={field('name')} disabled={pending} />
      <DescriptionField
        value={draft.description}
        onChange={field('description')}
        disabled={pending}
      />

      <section aria-labelledby="template-composition-heading" className="mt-8">
        <Typography variant="title" id="template-composition-heading">
          {t('create.body', { ns: 'templates' })}
        </Typography>
        <Typography variant="body" as="p" className="text-content-secondary max-w-measure mt-1">
          {t('screen.compositionHelp', { ns: 'templates' })}
        </Typography>
        <TemplateComposition
          value={draft.body}
          onChange={field('body')}
          disabled={pending}
          className="mt-3"
        />
      </section>

      {/* The state this screen has to report goes in one place, above the control that produced
          it, so a refusal is read where the thumb already is (design-language §4.3). */}
      {failed && <FieldMessage className="mt-auto pt-6">{errorMessage}</FieldMessage>}
      {saved && !failed && (
        <Typography variant="meta" as="p" role="status" className="mt-auto pt-6">
          {t('screen.saved', { ns: 'templates' })}
        </Typography>
      )}

      {/* Docked at every width, not only on the phone: the distance between the composition and
          the control that commits it is there on a desk too (§4.3). */}
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
        title={t('screen.leaveTitle', { ns: 'templates' })}
        confirmLabel={t('screen.leaveConfirm', { ns: 'templates' })}
        onClose={() => blocker.reset?.()}
        onConfirm={() => blocker.proceed?.()}
      >
        {t('screen.leaveDescription', { ns: 'templates' })}
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
      <Count left={left} />
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
      <Count left={left} />
    </div>
  )
}

function Count({ left }: { left: number }) {
  const { t } = useTranslation('common')
  return left < 0 ? (
    <FieldMessage role="status" className="mt-2">
      {t('count.exceeded', { count: -left })}
    </FieldMessage>
  ) : (
    <Typography variant="meta" as="p" className="mt-2">
      {t('count.remaining', { count: left })}
    </Typography>
  )
}
