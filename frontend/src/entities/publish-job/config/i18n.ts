import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    title: '발행하기',
    stage: {
      queued: 'Mac 연결을 기다리는 중',
      claimed: 'Mac에서 작업을 받았어요',
      preparing: '발행 준비 중',
      openingEditor: '네이버 편집기 여는 중',
      fillingContent: '글 입력 중',
      uploadingPhotos: '사진 올리는 중',
      fillingSettings: '태그·카테고리 설정 중',
      committing: '네이버에 최종 발행 중',
      verifying: '발행 결과 확인 중',
      published: '발행 완료',
      progress: '발행 진행 중',
    },
    startError: {
      aborted: '다른 화면에서 글이 바뀌었어요. 새로고침한 뒤 다시 확인해 주세요.',
      alreadyExists: '이미 진행 중이거나 발행을 마친 글이에요.',
      failedPrecondition: '글 확정, Mac 연결, 카테고리 상태를 확인해 주세요.',
      permissionDenied: '이 Mac 연결로는 발행할 수 없어요.',
      unknown: '발행 요청을 저장하지 못했어요. 다시 시도해 주세요.',
    },
  },
  en: {
    title: 'Publish',
    stage: {
      queued: 'Waiting for a Mac connection',
      claimed: 'The Mac accepted the job',
      preparing: 'Preparing to publish',
      openingEditor: 'Opening the Naver editor',
      fillingContent: 'Entering the post',
      uploadingPhotos: 'Uploading photos',
      fillingSettings: 'Entering tags and category',
      committing: 'Publishing to Naver',
      verifying: 'Verifying the published post',
      published: 'Published',
      progress: 'Publishing in progress',
    },
    startError: {
      aborted: 'The post changed in another screen. Refresh and review it again.',
      alreadyExists: 'This post is already being published or has been published.',
      failedPrecondition: 'Check the finalized post, Mac connection, and category.',
      permissionDenied: 'This Mac connection cannot publish the post.',
      unknown: 'Could not save the publishing request. Try again.',
    },
  },
} as const satisfies I18nFragment
