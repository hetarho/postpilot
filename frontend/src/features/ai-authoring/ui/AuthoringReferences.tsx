import { useId, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { FileText, Globe, LayoutTemplate, Paperclip, X } from 'lucide-react'
import {
  AUTHORING_MAX_REFERENCES,
  authoringReferenceKey,
  useCheckAuthoringReference,
  type AuthoringReference,
  type AuthoringReferenceRefusal,
} from '@/entities/ai-authoring'
import { PostPicker, displayTitle } from '@/entities/post'
import { useTemplates } from '@/entities/template'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Popover,
  SegmentedControl,
  TextField,
  Typography,
  typographyStyles,
} from '@/shared/ui'

type Source = AuthoringReference['kind']

const KIND_ICON = { post: FileText, template: LayoutTemplate, url: Globe } as const

/** Guide: what a template request follows besides the owner's words — one of their posts, one
 *  of their templates or a web page, at most three. Each attached one is a chip above the
 *  message field, named by its title and removable by its ✕; `참고 자료 붙이기` opens the app's
 *  own panel (a bottom sheet on a phone) to pick a post or a template by searching, or to type
 *  a page's address, which is read at once so a page that cannot be used says why in place.
 *
 *  Only a reference's form goes with the request, read on the server; nothing is stored on the
 *  template or the account, so the chips are this conversation's alone. */
export function AuthoringReferences({
  ownerId,
  references,
  onChange,
  disabled = false,
}: {
  ownerId: string
  references: readonly AuthoringReference[]
  onChange: (references: AuthoringReference[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation('authoring')
  const full = references.length >= AUTHORING_MAX_REFERENCES
  const add = (reference: AuthoringReference) => {
    const key = authoringReferenceKey(reference)
    if (full || references.some((attached) => authoringReferenceKey(attached) === key)) return
    onChange([...references, reference])
  }
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      {references.length > 0 && (
        <ul aria-label={t('references.attached')} className="flex min-w-0 flex-wrap gap-2">
          {references.map((reference) => {
            const Icon = KIND_ICON[reference.kind]
            const title =
              reference.title ||
              (reference.kind === 'url' ? reference.url : t('kinds.post-template'))
            return (
              <li
                key={authoringReferenceKey(reference)}
                className="bg-surface-raised flex max-w-full min-w-0 items-center gap-1.5 rounded-sm py-0.5 pl-3"
              >
                <Icon aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
                <span className="sr-only">{t(`references.kind.${reference.kind}`)}</span>
                <span
                  className={typographyStyles({
                    variant: 'meta',
                    className: 'text-content-primary min-w-0 truncate',
                  })}
                >
                  {title}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t('references.remove', { title })}
                  disabled={disabled}
                  onClick={() =>
                    onChange(
                      references.filter(
                        (attached) =>
                          authoringReferenceKey(attached) !== authoringReferenceKey(reference),
                      ),
                    )
                  }
                >
                  <X aria-hidden="true" className="size-4" />
                </Button>
              </li>
            )
          })}
        </ul>
      )}
      {full ? (
        <Typography variant="meta" as="p">
          {t('references.max', { max: AUTHORING_MAX_REFERENCES })}
        </Typography>
      ) : (
        <Popover
          label={t('references.panel')}
          triggerVariant="ghost"
          triggerSize="compact"
          triggerClassName="gap-1.5 px-3"
          triggerAttributes={{
            'aria-label':
              t('references.attach') +
              (references.length
                ? ' ' +
                  t('references.count', { count: references.length, max: AUTHORING_MAX_REFERENCES })
                : ''),
          }}
          triggerLabel={
            <>
              <Paperclip aria-hidden="true" className="size-4" />
              {t('references.attach')}
              {references.length > 0 && (
                <span className="text-content-tertiary">
                  {t('references.count', {
                    count: references.length,
                    max: AUTHORING_MAX_REFERENCES,
                  })}
                </span>
              )}
            </>
          }
          placement="above"
          align="start"
          phone="sheet"
          disabled={disabled}
        >
          {(close) => (
            <ReferencePanel
              ownerId={ownerId}
              onAdd={(reference) => {
                add(reference)
                close()
              }}
            />
          )}
        </Popover>
      )}
    </div>
  )
}

/** The panel: what is read, then 내 글 · 내 템플릿 · 웹 주소 and the chosen source's picker. */
function ReferencePanel({
  ownerId,
  onAdd,
}: {
  ownerId: string
  onAdd: (reference: AuthoringReference) => void
}) {
  const { t } = useTranslation('authoring')
  const [source, setSource] = useState<Source>('post')
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <Typography variant="meta" as="p" className="text-pretty">
        {t('references.help')}
      </Typography>
      <SegmentedControl
        value={source}
        onChange={(value) => setSource(value as Source)}
        ariaLabel={t('references.sources')}
        options={(['post', 'template', 'url'] as const).map((value) => ({
          value,
          label: t(`references.${value}`),
        }))}
      />
      {source === 'post' && (
        <PostPicker
          onPick={(post) => onAdd({ kind: 'post', slug: post.slug, title: displayTitle(post) })}
        />
      )}
      {source === 'template' && <TemplateChoices ownerId={ownerId} onAdd={onAdd} />}
      {source === 'url' && <AddressField onAdd={onAdd} />}
    </div>
  )
}

/** The owner's templates, narrowed by a search over their names. */
function TemplateChoices({
  ownerId,
  onAdd,
}: {
  ownerId: string
  onAdd: (reference: AuthoringReference) => void
}) {
  const { t } = useTranslation(['authoring', 'common'])
  const id = useId()
  const [search, setSearch] = useState('')
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const normalize = (value: string) =>
    value.normalize('NFKC').toLowerCase().trim().replace(/\s+/gu, ' ')
  const rows = templates.filter((template) =>
    normalize(template.name + ' ' + template.description).includes(normalize(search)),
  )
  return (
    <div>
      <FieldLabel htmlFor={id}>{t('references.searchTemplates')}</FieldLabel>
      <TextField
        id={id}
        type="search"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />
      {isError ? (
        <div role="alert" className="mt-3">
          <Typography variant="body">{t('references.templatesFailed')}</Typography>
          <Button variant="ghost" pending={isFetching} onClick={refetch}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </div>
      ) : isPending ? (
        <Typography variant="body" role="status" className="mt-3">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      ) : (
        <ul className="divide-divider mt-3 divide-y">
          {rows.map((template) => (
            <li key={template.id}>
              <Button
                variant="ghost"
                className="w-full justify-start text-left break-words"
                onClick={() => onAdd({ kind: 'template', id: template.id, title: template.name })}
              >
                <span className="min-w-0 break-words">{template.name}</span>
              </Button>
            </li>
          ))}
          {rows.length === 0 && (
            <li>
              <Typography variant="body">{t('references.noTemplates')}</Typography>
            </li>
          )}
        </ul>
      )}
    </div>
  )
}

/** A web page's address, read as soon as it is attached: a page that opens becomes a chip
 *  named by its title, and one that does not says why under the field, which keeps the
 *  address to correct. It is a field and a button rather than a form — the panel stands over
 *  the message field's own form, and Enter here must never send the message. */
function AddressField({ onAdd }: { onAdd: (reference: AuthoringReference) => void }) {
  const { t } = useTranslation('authoring')
  const id = useId()
  const check = useCheckAuthoringReference()
  const [address, setAddress] = useState('')
  const [pending, setPending] = useState(false)
  const [refusal, setRefusal] = useState<AuthoringReferenceRefusal | 'failed'>()
  const attach = async () => {
    const url = address.trim()
    if (!url || pending) return
    setPending(true)
    setRefusal(undefined)
    try {
      const read = await check({ kind: 'url', url, title: '' })
      if (read.refusal) setRefusal(read.refusal)
      else onAdd({ kind: 'url', url, title: read.title || url })
    } catch {
      setRefusal('failed')
    } finally {
      setPending(false)
    }
  }
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
    event.preventDefault()
    event.stopPropagation()
    void attach()
  }
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <FieldLabel htmlFor={id}>{t('references.urlLabel')}</FieldLabel>
      <TextField
        id={id}
        type="url"
        inputMode="url"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        placeholder={t('references.urlPlaceholder')}
        value={address}
        aria-invalid={!!refusal || undefined}
        aria-describedby={refusal ? id + '-refusal' : id + '-help'}
        onChange={(event) => {
          setAddress(event.target.value)
          setRefusal(undefined)
        }}
        onKeyDown={onKeyDown}
      />
      {refusal ? (
        <FieldMessage id={id + '-refusal'}>{t(`references.refusal.${refusal}`)}</FieldMessage>
      ) : (
        <Typography variant="meta" as="p" id={id + '-help'}>
          {pending ? t('references.checking') : t('references.urlHelp')}
        </Typography>
      )}
      <Button
        variant="secondary"
        className="self-start"
        pending={pending}
        disabled={!address.trim()}
        onClick={() => void attach()}
      >
        {t('references.urlAdd')}
      </Button>
    </div>
  )
}
