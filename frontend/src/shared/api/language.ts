import { ContentLanguage as ProtoContentLanguage } from './gen/postpilot/v1/language_pb'

export type ContentLanguage = 'ko' | 'en'

export const contentLanguages: readonly ContentLanguage[] = ['ko', 'en']

export function contentLanguageFromProto(value: ProtoContentLanguage): ContentLanguage | undefined {
  switch (value) {
    case ProtoContentLanguage.KOREAN:
      return 'ko'
    case ProtoContentLanguage.ENGLISH:
      return 'en'
    case ProtoContentLanguage.UNSPECIFIED:
    default:
      return undefined
  }
}

export function requireContentLanguage(value: ProtoContentLanguage): ContentLanguage {
  const language = contentLanguageFromProto(value)
  if (!language) {
    throw new Error(`unsupported content language enum: ${String(value)}`)
  }
  return language
}

export function contentLanguageToProto(language: ContentLanguage): ProtoContentLanguage {
  switch (language) {
    case 'ko':
      return ProtoContentLanguage.KOREAN
    case 'en':
      return ProtoContentLanguage.ENGLISH
  }
}
