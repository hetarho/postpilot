import type { PostDraft } from '@/entities/post'
import {
  boundedFreeText,
  CLIP_PROJECT_LIMITS,
  emptyClipProject,
  type ClipChannel,
  type ClipProject,
  type ClipProjectDraft,
  type ClipSourceCalls,
} from '@/entities/clip-project'
import {
  fetchClipSourceFile,
  transferClipSourceFiles,
  type ClipSourceImportResult,
} from '@/features/upload-clip-sources'

export interface CreatedPostClip {
  project: ClipProject
  sources: ClipSourceImportResult
}

/** The page composes the clip entity's CRUD hook with the browser upload action. */
export async function createClipFromPost({
  post,
  channel,
  create,
  sources,
  signal,
}: {
  post: PostDraft
  channel: ClipChannel
  create: (draft: ClipProjectDraft) => Promise<ClipProject>
  sources: ClipSourceCalls
  signal: AbortSignal
}): Promise<CreatedPostClip> {
  signal.throwIfAborted()
  if (!post.content) throw new Error('A clip needs a post with canonical content')
  const guide = post.advertiserGuide
  const project = await create({
    ...emptyClipProject(),
    channel,
    title: post.title.trim() ? post.title : post.content.title,
    instruction: boundedFreeText(post.memo, CLIP_PROJECT_LIMITS.instruction),
    campaign: post.campaign,
    advertiserGuide: {
      text: guide.text,
      keywords: [...guide.keywords],
      hashtags: [...guide.hashtags],
    },
  })
  signal.throwIfAborted()
  try {
    const imported = await transferClipSourceFiles(
      project.id,
      post.videos.map((video) => ({
        filename: video.filename,
        load: (signal: AbortSignal) =>
          fetchClipSourceFile(
            { url: video.viewUrl, filename: video.filename, contentType: video.contentType },
            signal,
          ),
      })),
      sources,
      signal,
    )
    return { project, sources: imported }
  } catch {
    signal.throwIfAborted()
    // Creation already committed. An uncertain reservation must never delete that clip
    // or its originals, nor cause a second creation when opening the result.
    return {
      project,
      sources: {
        uploaded: [],
        problems: post.videos.map((video) => ({ filename: video.filename, reason: 'unconfirmed' })),
        uncertain: true,
      },
    }
  }
}
