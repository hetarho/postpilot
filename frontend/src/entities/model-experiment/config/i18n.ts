import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16): the badge catalog's copy, which
 *  the sheet, the revealed comparison and the leaderboard all read from one place. */
export const i18n = {
  namespace: 'models',
  ko: {
    badge: {
      fast: '속도가 빨라요',
      natural: '자연스러워요',
      on_brief: '지시를 잘 지켰어요',
      structured: '구조가 좋아요',
      accurate: '내용이 정확해요',
      in_voice: '문체가 잘 맞아요',
      concise: '간결해요',
      slow: '느려요',
      ai_like: 'AI 같아요',
      off_brief: '지시를 벗어났어요',
      verbose: '장황해요',
      inaccurate: '내용이 틀렸어요',
      off_voice: '문체가 안 맞아요',
      repetitive: '반복이 많아요',
      broken_format: '형식이 깨졌어요',
      other: '기타',
    },
  },
  en: {
    badge: {
      fast: 'Fast',
      natural: 'Natural',
      on_brief: 'Followed the brief',
      structured: 'Well structured',
      accurate: 'Accurate',
      in_voice: 'In voice',
      concise: 'Concise',
      slow: 'Slow',
      ai_like: 'Sounds like AI',
      off_brief: 'Off brief',
      verbose: 'Verbose',
      inaccurate: 'Inaccurate',
      off_voice: 'Off voice',
      repetitive: 'Repetitive',
      broken_format: 'Broken format',
      other: 'Other',
    },
  },
} as const satisfies I18nFragment
