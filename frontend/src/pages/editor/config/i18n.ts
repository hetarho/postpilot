export const editorNavigationCopy = {
  ko: { home: '만들기로 돌아가기', history: '작업 내역으로 돌아가기', location: '글쓰기' },
  en: { home: 'Back to creation', history: 'Back to work history', location: 'Writing' },
}

export const clipFromPostCopy = {
  ko: {
    action: '이 체험으로 클립 만들기',
    confirm: '클립 만들기',
    open: '클립 열기',
    importing: '영상을 가져오는 중',
    channel: '클립 채널',
    description:
      '제목, 메모, 체험단 설정과 영상을 새 클립으로 가져와요. 사진은 글에 그대로 두고, 클립 설정에서 이어서 만들 수 있어요.',
    memoCut: (limit: number) =>
      `메모는 앞 ${limit}자까지 클립 지시문으로 가져와요. 나머지 메모는 글에 그대로 남아요.`,
  },
  en: {
    action: 'Make a clip from this experience',
    confirm: 'Create clip',
    open: 'Open clip',
    importing: 'Importing videos',
    channel: 'Clip channel',
    description:
      'Bring the title, memo, campaign settings and videos into a new clip. Photos stay with the post, and you can continue in the clip settings.',
    memoCut: (limit: number) =>
      `The first ${limit} characters of the memo become the clip instruction. The rest stays in the post.`,
  },
}
