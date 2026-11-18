import { useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { POSTS_SEARCH_DEBOUNCE_MS } from '@/shared/config'
import { Button, FieldLabel, TextField, Typography } from '@/shared/ui'
import { usePosts } from '../api/usePosts'
import { displayTitle, type PostListItem } from '../model/types'

/** Guide: one of the owner's posts, found by searching — a search field over their posts with
 *  content, title matches first, then the rest; a tap picks one. It is the one post picker: a
 *  clip's 글 연결 and a template request's 참고 자료 both use it, each keeping only what a pick
 *  does. `initialQuery` opens it already narrowed. */
export function PostPicker({
  initialQuery = '',
  pending = false,
  onPick,
}: {
  initialQuery?: string
  pending?: boolean
  onPick: (post: PostListItem) => void
}) {
  const { t } = useTranslation('posts')
  const searchId = useId()
  const [search, setSearch] = useState(initialQuery)
  const [settled, setSettled] = useState(initialQuery)
  useEffect(() => {
    if (search === settled) return
    const timer = setTimeout(() => setSettled(search), POSTS_SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [search, settled])
  const matching = usePosts(settled)
  const all = usePosts()
  const seen = new Set<string>()
  // POST-91 also matches tags; only actual title matches get priority here.
  const normalize = (value: string) =>
    value.normalize('NFKC').toLowerCase().trim().replace(/\s+/gu, ' ')
  const titleMatches = matching.posts.filter((post) =>
    normalize(post.title).includes(normalize(settled)),
  )
  const rows = [...titleMatches, ...all.posts].filter((post) => {
    if (!post.contentReady || seen.has(post.slug)) return false
    seen.add(post.slug)
    return true
  })
  return (
    <div>
      <FieldLabel htmlFor={searchId}>{t('picker.search')}</FieldLabel>
      <TextField
        id={searchId}
        type="search"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />
      {matching.isError || all.isError ? (
        <div role="alert" className="mt-3">
          <Typography variant="body">{t('picker.loadFailed')}</Typography>
          <Button
            variant="ghost"
            onClick={() => {
              matching.refetch()
              all.refetch()
            }}
          >
            {t('picker.retry')}
          </Button>
        </div>
      ) : matching.isPending || all.isPending || search !== settled ? (
        <Typography variant="body" role="status" className="mt-3">
          {t('picker.loading')}
        </Typography>
      ) : (
        <ul className="divide-divider mt-3 divide-y">
          {rows.map((post) => (
            <li key={post.slug}>
              <Button
                variant="ghost"
                className="w-full justify-start text-left break-words"
                disabled={pending}
                onClick={() => onPick(post)}
              >
                <span className="min-w-0 break-words">{displayTitle(post)}</span>
              </Button>
            </li>
          ))}
          {rows.length === 0 && (
            <li>
              <Typography variant="body">{t('picker.none')}</Typography>
            </li>
          )}
        </ul>
      )}
    </div>
  )
}
