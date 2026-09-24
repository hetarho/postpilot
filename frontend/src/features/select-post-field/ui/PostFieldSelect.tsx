import { useEffect, useId, useRef, useState, type ComponentType } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Baby,
  Ban,
  ChevronDown,
  Coffee,
  MapPinned,
  NotebookPen,
  Package,
  PawPrint,
  Sofa,
  Sparkles,
  UtensilsCrossed,
} from 'lucide-react'
import {
  BLOG_FIELD_IDS,
  NO_BLOG_FIELD,
  blogFieldLabelKey,
  type BlogFieldChoice,
} from '@/entities/blog-field'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import {
  ChipToggle,
  FieldLabel,
  FieldMessage,
  Popover,
  Typography,
  type PopoverHandle,
} from '@/shared/ui'

/** The glyph each choice wears beside its name, in the trigger and in the panel alike. */
const ICONS: Record<BlogFieldChoice, ComponentType<{ className?: string }>> = {
  [NO_BLOG_FIELD]: Ban,
  restaurant: UtensilsCrossed,
  cafe: Coffee,
  domestic_travel: MapPinned,
  fashion_beauty: Sparkles,
  product_review: Package,
  parenting_marriage: Baby,
  pets: PawPrint,
  interior_diy: Sofa,
  daily_life: NotebookPen,
}

const CHOICES: readonly BlogFieldChoice[] = [NO_BLOG_FIELD, ...BLOG_FIELD_IDS]

interface PostFieldSelectProps {
  /** The 분야 as the editor shows it; '' is 없음. */
  value: BlogFieldChoice
  /** Off for a reason the caller states elsewhere, so this field adds none of its own. */
  disabled?: boolean
  /** Saves the choice: through the draft queue on a saved post, into local state on /posts/new. A
   *  rejection is shown under the field; taking the choice back is the caller's. */
  onSelect: (choice: BlogFieldChoice) => Promise<void> | void
  className?: string
}

/** The post's optional 분야 (POST-82), defaulting to 없음. The trigger wears the current choice —
 *  its glyph, its name and a chevron — and opens a popover of every choice as a glyph-and-name
 *  chip, exactly one of them pressed; pressing another takes the choice and closes the panel
 *  (owner decision 2026-09-24). Ten short names read at a glance as a set of chips, where a
 *  dropdown made them a column to scan. The value is read when a run is enqueued, so it stays
 *  usable while a job runs — the job keeps the 분야 it started with. */
export function PostFieldSelect({
  value,
  disabled = false,
  onSelect,
  className,
}: PostFieldSelectProps) {
  const { t } = useTranslation('posts')
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  const popoverRef = useRef<PopoverHandle>(null)
  // The chip that was pressed unmounts with the panel, and the trigger it would hand focus back
  // to is disabled while the choice saves — so focus returns once the save has settled.
  const refocus = useRef(false)
  const label = t('postField.label')
  const Current = ICONS[value]

  useEffect(() => {
    if (pending || !refocus.current) return
    refocus.current = false
    popoverRef.current?.focus()
  }, [pending])

  const select = async (next: BlogFieldChoice) => {
    if (next === value) return
    setFailure(undefined)
    refocus.current = true
    setPending(true)
    try {
      await onSelect(next)
    } catch (cause) {
      setFailure(appFailureFromConnect(cause))
    } finally {
      setPending(false)
    }
  }

  return (
    <div className={className}>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Popover
        ref={popoverRef}
        label={label}
        triggerAttributes={{
          id,
          'aria-label': `${label} ${t(blogFieldLabelKey(value))}`,
          'aria-describedby': `${hintId}${failure ? ` ${errorId}` : ''}`,
        }}
        triggerLabel={
          <>
            <Current aria-hidden="true" className="size-4 shrink-0" />
            <span className="min-w-0 truncate">{t(blogFieldLabelKey(value))}</span>
            <ChevronDown aria-hidden="true" className="size-4 shrink-0" />
          </>
        }
        disabled={disabled || pending}
        // A bottom sheet on a phone: the room under a field in ①'s flow depends on the scroll,
        // and the tab bar and the dock cover the bottom of it, which cut the last row of chips
        // off. From `sm:` it opens downward from its own left edge.
        phone="sheet"
        placement="below"
        align="start"
        className="mt-1 max-w-full"
        triggerClassName="max-w-full"
      >
        {(close) => (
          <div role="group" aria-label={label} className="flex flex-wrap gap-2">
            {CHOICES.map((choice) => {
              const Icon = ICONS[choice]
              const pressed = choice === value
              return (
                <ChipToggle
                  key={choice || 'none'}
                  pressed={pressed}
                  data-autofocus={pressed || undefined}
                  className="gap-1.5"
                  onClick={() => {
                    close()
                    void select(choice)
                  }}
                >
                  <Icon aria-hidden="true" className="size-4 shrink-0" />
                  {t(blogFieldLabelKey(choice))}
                </ChipToggle>
              )
            })}
          </div>
        )}
      </Popover>
      <Typography variant="body" as="p" id={hintId} className="text-content-secondary mt-2">
        {t('postField.help')}
      </Typography>
      {failure && (
        <FieldMessage id={errorId} className="mt-2">
          {formatAppFailure(failure)}
        </FieldMessage>
      )}
    </div>
  )
}
