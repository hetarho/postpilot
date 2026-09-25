import { useId, type ComponentType } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Baby,
  Ban,
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
import { ChipToggle, Typography } from '@/shared/ui'

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
  /** The 분야 as the form holds it; '' is 없음. */
  value: BlogFieldChoice
  /** Reports a chip as the choice; the brief's 저장 saves it with the rest of the set (POST-89). */
  onChange: (choice: BlogFieldChoice) => void
  /** Off for a reason the caller states elsewhere, so this field adds none of its own. */
  disabled?: boolean
  className?: string
}

/** The post's optional 분야 (POST-82), defaulting to 없음, a control of the writing brief's
 *  run-options form. Every choice is a glyph-and-name chip, exactly one of them pressed (owner
 *  decision 2026-09-24). They render IN PLACE under the field's name, with no trigger and no
 *  popover of their own: the brief is itself a popover (a sheet on a phone), a popover inside its
 *  scrolling panel would be clipped by it, and one Escape would close the brief too and discard
 *  the unsaved form. The value is read when a run is enqueued, so it stays usable while a job
 *  runs — the job keeps the 분야 it started with. */
export function PostFieldSelect({
  value,
  onChange,
  disabled = false,
  className,
}: PostFieldSelectProps) {
  const { t } = useTranslation('posts')
  const labelId = useId()
  return (
    <div className={className}>
      <Typography variant="label" as="p" id={labelId}>
        {t('postField.label')}
      </Typography>
      <div role="group" aria-labelledby={labelId} className="mt-2 flex flex-wrap gap-2">
        {CHOICES.map((choice) => {
          const Icon = ICONS[choice]
          const pressed = choice === value
          return (
            <ChipToggle
              key={choice || 'none'}
              pressed={pressed}
              disabled={disabled}
              className="gap-1.5"
              onClick={() => {
                if (!pressed) onChange(choice)
              }}
            >
              <Icon aria-hidden="true" className="size-4 shrink-0" />
              {t(blogFieldLabelKey(choice))}
            </ChipToggle>
          )
        })}
      </div>
    </div>
  )
}
