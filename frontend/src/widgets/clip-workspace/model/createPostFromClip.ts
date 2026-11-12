import { copyGuide, EMPTY_GUIDE } from '@/entities/advertiser-guide'
import type { ClipProject } from '@/entities/clip-project'
import type { PostDraftSave } from '@/entities/post'
import type { ContentLanguage } from '@/shared/api'

/** ③'s `이 체험으로 글 쓰기` (CLIP-233), the reverse of a post's `이 체험으로 클립 만들기`: one
 *  create seeded with the clip's title as the 가제, its instruction as the memo and its 체험단 글
 *  switch and advertiser guide, in the clip's language, on the account's made 기본 voice as any new
 *  post starts (POST-101). No source or result video is copied and nothing on the clip changes;
 *  the post's slug is what opens it. */
export function createPostFromClip({
  project,
  fallbackLanguage,
  voiceId,
  save,
}: {
  project: Pick<ClipProject, 'title' | 'instruction' | 'campaign' | 'advertiserGuide' | 'language'>
  /** The language a clip that recorded none is read in. */
  fallbackLanguage: ContentLanguage
  /** The account's made 기본 voice; absent creates the post in 말투 없음. */
  voiceId?: string
  save: (draft: PostDraftSave) => Promise<string>
}): Promise<string> {
  const guide = project.advertiserGuide ?? EMPTY_GUIDE
  // An empty slug is a create (POST-4).
  return save({
    slug: '',
    title: project.title,
    memo: project.instruction ?? '',
    templateAnswers: [],
    ...(voiceId ? { voiceId } : {}),
    targetLanguage: project.language ?? fallbackLanguage,
    campaign: project.campaign ?? false,
    advertiserGuide: copyGuide({
      ...EMPTY_GUIDE,
      text: guide.text,
      keywords: guide.keywords,
      hashtags: guide.hashtags,
    }),
  })
}
