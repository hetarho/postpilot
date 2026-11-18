import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Clapperboard, PenLine } from 'lucide-react'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import { CLIP_LIST_PATH, rememberClipEntry } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { ChoiceRow, ListGroup } from '@/shared/ui'
import { penIllustration, videoIllustration } from '@/shared/assets'
import {
  CREATION_MODES,
  rememberCreationMode,
  useCreationMode,
  type CreationMode,
} from '../model/creation-mode'
import { KitDropdown } from './KitDropdown'

const START: Record<CreationMode, { to: '/posts/new' | '/clips/new'; picture: string }> = {
  post: { to: '/posts/new', picture: penIllustration },
  video: { to: '/clips/new', picture: videoIllustration },
}

/** Guide: the middle of 홈's picture — the screen's one way to start making something, the way
 *  every piece around it works: its icon starts it, and its name is a dropdown that switches what
 *  it starts between 글쓰기 and 영상 만들기. The switch is remembered for the account on this
 *  browser, so 홈 opens on the last one chosen.
 *
 *  On the orbit it is the ringed disc, the name inside it under the picture; where the picture
 *  falls back to a column it is a row like the pieces', its icon on the CTA's own fill. */
export function CreationCentre({ complete, fill }: { complete: boolean; fill: number }) {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const mode = useCreationMode(ownerId)
  const name = t(`kit.mode.names.${mode}`)
  const Icon = mode === 'video' ? Clapperboard : PenLine
  const remember = () => {
    if (mode === 'video')
      rememberClipEntry(ownerId, {
        path: CLIP_LIST_PATH,
        section: 'video',
        filters: {},
        scrollY: 0,
      })
    else
      rememberPostEntry(ownerId, {
        path: POST_LIST_PATH,
        section: 'post',
        filters: {},
        scrollY: 0,
      })
  }
  return (
    <div className="relative @xs:absolute @xs:top-1/2 @xs:left-1/2 @xs:-translate-1/2">
      {/* The halo: it glows with how much of the kit is there, and breathes once all of it is. */}
      <span
        aria-hidden="true"
        style={{ opacity: complete ? undefined : 0.12 + fill * 0.5 }}
        className={clsx(
          'bg-promo-stroke pointer-events-none absolute inset-0 hidden rounded-full blur-2xl @xs:block @xs:scale-150',
          complete && 'animate-promo-glow',
        )}
      />
      <div className="duration-base ease-standard relative flex min-h-16 items-center gap-3 px-3 transition-transform @xs:size-24 @xs:flex-col @xs:justify-center @xs:gap-0 @xs:rounded-full @xs:px-0 @xs:has-[>a:hover]:scale-105 @md:size-32">
        <Link
          to={START[mode].to}
          search={{}}
          onClick={remember}
          aria-label={name}
          className={clsx(
            'bg-button-cta-bg text-button-cta-fg hover:bg-button-cta-bg-hover active:bg-button-cta-bg-active duration-base ease-standard flex size-14 shrink-0 items-center justify-center rounded-full shadow-lg transition-colors',
            // On the orbit it IS the disc, its ring the disc's border; the picture and the name
            // sit over it. The app's own focus outline (unlayered, hence the `!`) stands clear of
            // that ring, where a ring-coloured focus would not show.
            '@xs:bg-surface-raised @xs:ring-button-cta-bg @xs:hover:bg-surface-raised @xs:active:bg-surface-raised @xs:absolute @xs:inset-0 @xs:size-auto @xs:ring-4 @xs:focus-visible:outline-offset-8!',
          )}
        >
          <Icon aria-hidden="true" className="size-6 @xs:hidden" />
        </Link>
        <img
          src={START[mode].picture}
          alt=""
          aria-hidden="true"
          className={clsx(
            'pointer-events-none relative hidden -translate-y-0.5 object-contain drop-shadow-md @xs:block @md:size-20',
            // 영상 만들기 takes two lines in the smaller disc, so its picture gives them the room.
            mode === 'video' ? 'size-11' : 'size-14',
          )}
        />
        <span className="relative max-w-full min-w-0 @xs:-mt-1">
          <KitDropdown
            name={name}
            label={t('kit.mode.title')}
            triggerLabel={t('kit.mode.change', { mode: name })}
            triggerClassName="@xs:text-badge-accent-fg @xs:px-1"
            wrap
            title={t('kit.mode.title')}
            help={t('kit.mode.help')}
          >
            {(chosen) => (
              <div role="radiogroup" aria-label={t('kit.mode.title')}>
                <ListGroup>
                  {CREATION_MODES.map((choice) => (
                    <li key={choice} data-autofocus={choice === mode || undefined}>
                      <ChoiceRow
                        mode="radio"
                        checked={choice === mode}
                        name={t(`kit.mode.names.${choice}`)}
                        meta={t(`kit.mode.meta.${choice}`)}
                        onPick={() => {
                          rememberCreationMode(ownerId, choice)
                          chosen()
                        }}
                      />
                    </li>
                  ))}
                </ListGroup>
              </div>
            )}
          </KitDropdown>
        </span>
      </div>
    </div>
  )
}
