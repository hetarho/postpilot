import { clone, create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  appFailureFromConnect,
  contentLanguageToProto,
  type ContentLanguage,
  type GetPostResponse,
  GetPostResponseSchema,
  type Post,
  PostSchema,
  PostService,
  StorylineEditSchema,
  StorylineParagraphSchema,
  StorylineSchema,
  TemplateAnswerSchema,
  TemplateRefSchema,
  VoiceRefSchema,
} from '@/shared/api'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'

/** Applies only the fields this mutation owns to the cached post.
 *
 *  SavePostDraft answers with a whole post snapshot, but the request is in flight while
 *  uploads and generation independently advance images, observations, active_job and
 *  content. Installing that snapshot wholesale could roll any of them back in the cache.
 *  Title, memo and the assignments are the fields this mutation settles; every other
 *  field remains owned by GetPost or its focused mutation patch — a reassignment included: it
 *  changes the voice and nothing else, leaving the baseline, the revision and the status as
 *  they were (POST-24). */
export function applyingSavedDraft(
  saved: Post,
  cached: GetPostResponse | undefined,
  { carriedStoryline = false }: { carriedStoryline?: boolean } = {},
): Post {
  if (!cached?.post) return saved
  const post = clone(PostSchema, cached.post)
  post.title = saved.title
  post.memo = saved.memo
  post.targetLanguage = saved.targetLanguage
  // Unconditional, like the 템플릿 below: the response always reports the current voice, and an
  // unset one is a real answer (말투 없음, POST-25). A `if (saved.voice)` guard would make a clear
  // invisible until the next GetPost.
  post.voice = saved.voice ? clone(VoiceRefSchema, saved.voice) : undefined
  // An ASSIGNMENT seeds the post's two generation options from the template it assigns
  // (TMPL-48), so the values that come back with it are the ones this mutation settled.
  // Only then: an ordinary autosave of title and memo carries whatever the row held when the
  // request was built, and installing that would roll back an options save that landed while
  // it was in flight. Compared BEFORE `post.template` is overwritten below, or it always
  // reads equal.
  if (saved.template?.id !== cached.post.template?.id) {
    post.targetLength = saved.targetLength
    post.tagCount = saved.tagCount
  }
  // Unconditional for the same reason as the voice: an unset 템플릿 is 없음.
  post.template = saved.template ? clone(TemplateRefSchema, saved.template) : undefined
  // The 분야 is the brief's options save's now (POST-89), like the two numbers above: an ordinary
  // autosave answers with the 분야 the row held when it was read, and installing that would roll
  // back an options save that landed while it was out.
  // Unconditional for the same reason: the response always reports the post's whole answer
  // set, and a save that cleared one has to be visible before the next GetPost (POST-62).
  post.templateAnswers = saved.templateAnswers.map((answer) => clone(TemplateAnswerSchema, answer))
  // The storyline only from a save that carried an edit of it (POST-96): an ordinary autosave
  // answers with the storyline the row held when it was read, and installing that could roll back
  // one a storyline job wrote while the save was out.
  if (carriedStoryline)
    post.storyline = saved.storyline ? clone(StorylineSchema, saved.storyline) : undefined
  return post
}

/** Create-or-update for a draft: an empty slug creates the post and returns the minted
 *  one (POST-4). This is the autosave endpoint, so it is called about once
 *  a second while someone types. */
/** One draft save, in the editor's terms. An assignment member left undefined leaves the post's
 *  value alone; '' clears a voice to 말투 없음 and a 템플릿 to 없음. There is no 분야 member: this build saves the 분야 with the
 *  brief's run options (POST-89), so no autosave can carry one. */
export interface PostDraftSave {
  /** Empty for the save that creates the post. */
  slug: string
  title: string
  memo: string
  templateAnswers: readonly { label: string; text: string; enabled: boolean }[]
  voiceId?: string
  templateId?: string
  targetLanguage?: ContentLanguage
  /** The owner's storyline edit, the whole paragraph list; absent keeps the stored one. */
  storyline?: readonly { text: string; files: readonly string[] }[]
}

export function useSavePostDraft(): { save: (draft: PostDraftSave) => Promise<string> } {
  const queryClient = useQueryClient()
  // The transport the hooks are mounted on — the same one the keys must be built from.
  const transport = useTransport()

  const mutation = useMutation(PostService.method.savePostDraft, {
    onSuccess: (data, variables) => {
      const post = data.post
      if (!post) return

      // Seeding matters for more than a saved round trip: the first save of a new post
      // moves the URL to the minted slug, and the editor that route mounts reads this
      // entry. Without it the user would watch the text they just typed disappear and
      // come back.
      const key = getPostQueryKey(transport, post.slug)
      queryClient.setQueryData(
        key,
        create(GetPostResponseSchema, {
          post: applyingSavedDraft(post, queryClient.getQueryData<GetPostResponse>(key), {
            carriedStoryline: Boolean(variables.storyline),
          }),
        }),
      )

      // The list is ordered by updated_at and shows the title, so every save changes it.
      // Marking it stale is enough — the list is on another route, and react-query
      // refetches an inactive query when it is next mounted rather than now.
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
    // Refused because the post was published elsewhere (POST-86): the cached post still reads
    // writable, so refetch it and the list for ① to re-render locked.
    onError: (error, variables) => {
      if (!variables.slug || appFailureFromConnect(error).reason !== 'POST_PUBLISHED_LOCKED') return
      void queryClient.invalidateQueries({ queryKey: getPostQueryKey(transport, variables.slug) })
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
    },
  })

  return {
    /** Resolves with the saved post's slug. */
    save: async (draft) => {
      const response = await mutation.mutateAsync({
        slug: draft.slug,
        title: draft.title,
        memo: draft.memo,
        templateAnswers: draft.templateAnswers.map((answer) => ({ ...answer })),
        voiceId: draft.voiceId,
        templateId: draft.templateId,
        targetLanguage:
          draft.targetLanguage === undefined
            ? undefined
            : contentLanguageToProto(draft.targetLanguage),
        storyline: draft.storyline
          ? create(StorylineEditSchema, {
              paragraphs: draft.storyline.map((paragraph) =>
                create(StorylineParagraphSchema, {
                  text: paragraph.text,
                  files: [...paragraph.files],
                }),
              ),
            })
          : undefined,
      })
      // A 200 carrying no post is not a confirmation. Taking it as one would mark the text
      // saved, and for a draft with no slug yet would leave the next edit creating a second post.
      if (!response.post?.slug) throw new Error('SavePostDraft returned no post')
      return response.post.slug
    },
  }
}
