import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import type { PostDraft } from '@/entities/post'
import { SaveStatus, peekPendingDraft, useAutosave } from '@/features/save-draft'
import { FieldLabel, Textarea, TextField } from '@/shared/ui'
import { clearCaret, peekCaret, stashCaret } from '../model/editor-handoff'
import { EditorPhotos } from './EditorPhotos'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
  status?: ReactNode
}

/** Title + memo, autosaved. The screen the whole input side of the product hangs off
 *  (PRD F-2); photos, generation, revision and export all extend it in later plans. */
export function DraftEditor({ post, status }: DraftEditorProps) {
  const navigate = useNavigate()
  const titleRef = useRef<HTMLInputElement>(null)
  const memoRef = useRef<HTMLTextAreaElement>(null)

  // Text still queued for this post outranks what the server reported: it is what the
  // previous editor was in the middle of saving when the mint moved the URL, so it is
  // newer by exactly the characters typed during that round trip.
  const opening = post
    ? (peekPendingDraft(post.slug) ?? { title: post.title, memo: post.memo })
    : { title: '', memo: '' }
  const [title, setTitle] = useState(opening.title)
  const [memo, setMemo] = useState(opening.memo)

  // Read, not consumed — a component body may run more than once per mount.
  const caret = post ? peekCaret(post.slug) : undefined

  const autosave = useAutosave({
    post,
    title,
    memo,
    onMinted: (slug) => {
      // Read off the live DOM, so this is the caret as it is now rather than as it was
      // when the save left.
      const focused = document.activeElement
      const field =
        focused === titleRef.current ? 'title' : focused === memoRef.current ? 'memo' : undefined
      if (field) {
        const element = field === 'title' ? titleRef.current : memoRef.current
        stashCaret({
          slug,
          field,
          selectionStart: element?.selectionStart ?? 0,
          selectionEnd: element?.selectionEnd ?? 0,
        })
      }
      // `replace`, so the back button goes to the list rather than to /posts/new — which
      // would open a second empty draft.
      void navigate({ to: '/posts/$slug', params: { slug }, replace: true })
    },
  })

  useEffect(() => {
    if (!caret) return
    clearCaret()
    const element = caret.field === 'title' ? titleRef.current : memoRef.current
    if (!element) return
    element.focus()
    element.setSelectionRange(caret.selectionStart, caret.selectionEnd)
  }, [caret])

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-6 sm:px-6">
      <div className="flex items-center justify-between">
        <Link
          to="/posts"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
        >
          ← 글 목록
        </Link>
        <SaveStatus state={autosave.state} />
      </div>

      {status && <div className="mt-4">{status}</div>}

      <FieldLabel htmlFor="post-title" className="sr-only">
        제목
      </FieldLabel>
      <TextField
        id="post-title"
        ref={titleRef}
        appearance="bare"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        placeholder="제목"
        className="mt-4 min-h-11 text-2xl font-semibold tracking-tight"
      />

      <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />

      <FieldLabel htmlFor="post-memo" className="sr-only">
        메모
      </FieldLabel>
      <Textarea
        id="post-memo"
        ref={memoRef}
        appearance="bare"
        value={memo}
        onChange={(event) => setMemo(event.target.value)}
        placeholder="무슨 일이 있었는지 편하게 적어 주세요"
        rows={16}
        className="mt-5 text-sm leading-relaxed"
      />
    </main>
  )
}
