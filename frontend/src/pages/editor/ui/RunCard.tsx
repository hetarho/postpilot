import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Eye, PenLine } from 'lucide-react'
import { progressRatio, type GenerationJob } from '@/entities/generation-job'
import { Typography, cardStyles } from '@/shared/ui'

/** The run in progress, heading ② where an observation, a storyline or a writing run started from
 *  ① or ② is waited on (POST-125). The page-top hairline and the status line carry the same job;
 *  this is the one place it is big enough to wait on.
 *  While it is in view it is the ONE bar the owner sees: it reports itself so the hairline fades
 *  out, and back in once the card has scrolled away (or the run ends and the card goes). */
export function RunCard({
  job,
  onShown,
}: {
  job: GenerationJob
  onShown?: (shown: boolean) => void
}) {
  const { t } = useTranslation('posts')
  const ratio = progressRatio(job)
  const share = ratio ? Math.round((ratio.done / Math.max(ratio.total, 1)) * 100) : undefined
  const card = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const node = card.current
    if (!node || !onShown || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(
      (entries) => onShown(entries.some((entry) => entry.isIntersecting)),
      { threshold: 0.5 },
    )
    observer.observe(node)
    return () => {
      observer.disconnect()
      onShown(false)
    }
  }, [onShown])
  return (
    <div
      ref={card}
      className={cardStyles({
        className: 'bg-badge-accent-bg flex items-center gap-4 shadow-none',
      })}
    >
      {/* A still mark, not a spinner: the bar below is the card's one moving progress. A spinner
          beside it said "working" twice, and while the write stage has no ratio both of them
          swept with nothing to tell apart. */}
      <span
        aria-hidden="true"
        className="text-badge-accent-fg flex size-12 shrink-0 items-center justify-center rounded-full"
      >
        {job.stage === 'observe' ? <Eye className="size-7" /> : <PenLine className="size-7" />}
      </span>
      <span className="min-w-0 flex-1">
        {/* The stage itself is the status line's, at the top; this card only says it is working
            and how far along. */}
        <Typography variant="fieldTitle" as="span" className="block">
          {t(job.stage === 'observe' ? 'guide.observing' : 'guide.writing')}
        </Typography>
        <span
          aria-hidden="true"
          className="bg-meter-track mt-3 block h-2 overflow-hidden rounded-full"
        >
          {share === undefined ? (
            <span className="bg-meter-fill animate-progress-sweep block h-full w-1/4 rounded-full" />
          ) : (
            // A full-width fill slid in from the left, so the bar moves on `transform` alone
            // (THEME-28) and its leading end keeps its round cap, which a scale would squash.
            <span
              data-writing-fill
              className="bg-meter-fill duration-slow ease-standard block h-full w-full rounded-full transition-transform"
              style={{ transform: `translateX(${share - 100}%)` }}
            />
          )}
        </span>
      </span>
    </div>
  )
}
