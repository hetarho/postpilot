import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    export: {
      videoHint: '기기의 원본 영상을 편집기에서 직접 첨부해 주세요',
      title: '내보내기',
      format: '내보내기 형식',
      result: '내보내기 결과',
      naverTitle: '네이버 제목',
      copyTitle: '제목 복사',
      titleCopied: '제목이 복사됐어요',
      manualCopy: '자동 복사가 막혀 있어요. 선택된 텍스트를 길게 눌러 복사하세요',
      tags: '태그',
      copyTags: '태그 복사',
      tagsCopied: '태그가 복사됐어요',
      preview: '네이버 미리보기',
      photoCopyAria: '{{number}}번 사진 복사 · {{file}}',
      photoCopied: '{{file}} 사진이 복사됐어요',
      captionCopyAria: '{{number}}번 사진 캡션 복사',
      captionCopied: '캡션이 복사됐어요',
      captionField: '캡션 텍스트',
      photoUnsupported:
        '이 브라우저는 사진 복사를 지원하지 않아요. 본문만 붙여넣고 사진은 직접 올려 주세요.',
      photoRefused: '사진 복사가 막혔어요. 다시 시도해 주세요.',
      photoBlocked: '지금은 사진을 복사할 수 없어요. 본문만 붙여넣고 사진은 직접 올려 주세요.',
      photoUnreadable: '사진을 읽지 못했어요. 글을 다시 불러오면 사진 주소가 새로 발급돼요.',
      photoMissing: '이 표시에 해당하는 사진을 찾지 못했어요.',
      languageMissing: '글의 내용 언어 정보가 없어 내보낼 수 없어요. 글을 다시 불러와 주세요.',
      formatLabel: {
        naver: '네이버 블로그',
        tistory: '티스토리',
        site: '자체 사이트',
        markdown: '마크다운',
      },
      guidance: {
        naver: '본문을 그대로 붙여넣으세요',
        naverPhotos:
          '본문을 붙여넣은 뒤, 사진_1_사진 같은 자리마다 미리보기의 사진을 복사해 넣으세요. 캡션은 본문에 들어 있지 않으니 캡션도 따로 복사해 편집기의 캡션 칸에 넣어 주세요',
        tistory: 'HTML 모드에 붙여넣고 사진 업로드 후 src를 교체하세요',
        site: '그대로 .html로 저장하고 사진 파일을 옆에 두세요',
        markdown: 'Hugo · Jekyll · Obsidian에 맞는 형식이에요. 사진 파일을 같은 폴더에 두세요',
      },
    },
  },
  en: {
    export: {
      videoHint: 'Attach the original video from your device in the editor',
      title: 'Export',
      format: 'Export format',
      result: 'Export result',
      naverTitle: 'Naver title',
      copyTitle: 'Copy title',
      titleCopied: 'Title copied',
      manualCopy: 'Automatic copying is blocked. Press and hold the selected text to copy it',
      tags: 'Tags',
      copyTags: 'Copy tags',
      tagsCopied: 'Tags copied',
      preview: 'Naver preview',
      photoCopyAria: 'Copy photo {{number}} · {{file}}',
      photoCopied: 'Copied the photo {{file}}',
      captionCopyAria: 'Copy the caption of photo {{number}}',
      captionCopied: 'Caption copied',
      captionField: 'Caption text',
      photoUnsupported:
        'This browser cannot copy images. Paste the text and add the photos yourself.',
      photoRefused: 'The photo copy was blocked. Try again.',
      photoBlocked:
        'The photo cannot be copied right now. Paste the text and add the photos yourself.',
      photoUnreadable: 'Could not read the photo. Reload the post to mint a fresh photo URL.',
      photoMissing: 'No photo matches this marker.',
      languageMissing:
        'This post has no content-language provenance, so it cannot be exported. Reload the post and try again.',
      formatLabel: {
        naver: 'Naver Blog',
        tistory: 'Tistory',
        site: 'Own site',
        markdown: 'Markdown',
      },
      guidance: {
        naver: 'Paste the text as it is',
        naverPhotos:
          "Paste the text, then replace each photo_1_photo marker with the matching photo from the preview. The captions are not in the pasted text — copy each one and put it in the editor's caption box",
        tistory: 'Paste in HTML mode, upload the photos, then replace each src',
        site: 'Save it as an .html file and place the photo files beside it',
        markdown: 'For Hugo · Jekyll · Obsidian. Place the photo files in the same folder',
      },
    },
  },
} as const satisfies I18nFragment
