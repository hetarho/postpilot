export const editorNavigationCopy = {
  ko: {
    /** The 글 tab, where a post lives. */
    home: '글로 돌아가기',
    /** A return remembered from the retired 내 콘텐츠 hub, which now opens 글. */
    mine: '글로 돌아가기',
    location: '글쓰기',
  },
  en: { home: 'Back to posts', mine: 'Back to posts', location: 'Writing' },
}

export const clipFromPostCopy = {
  ko: {
    action: '이 체험으로 영상 만들기',
    confirm: '영상 만들기',
    open: '영상 열기',
    importing: '영상을 가져오는 중',
    description:
      '제목, 메모, 체험단 설정과 영상을 새 영상 작업으로 가져와요. 사진은 글에 그대로 두고, 영상 설정에서 이어서 만들 수 있어요.',
    memoCut: (limit: number) =>
      `메모는 앞 ${limit}자까지 영상 지시문으로 가져와요. 나머지 메모는 글에 그대로 남아요.`,
  },
  en: {
    action: 'Make a video from this experience',
    confirm: 'Create video',
    open: 'Open video',
    importing: 'Importing videos',
    description:
      'Bring the title, memo, campaign settings and videos into a new clip. Photos stay with the post, and you can continue in the clip settings.',
    memoCut: (limit: number) =>
      `The first ${limit} characters of the memo become the clip instruction. The rest stays in the post.`,
  },
}
