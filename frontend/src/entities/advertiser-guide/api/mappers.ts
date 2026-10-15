import { create } from '@bufbuild/protobuf'
import { AdvertiserGuideSchema, type ProtoAdvertiserGuide } from '@/shared/api'
import type { AdvertiserGuide } from '../model/types'

export function advertiserGuideFromProto(value: ProtoAdvertiserGuide | undefined): AdvertiserGuide {
  return {
    text: value?.text ?? '',
    keywords: [...(value?.keywords ?? [])],
    minChars: value?.minChars,
    minPhotos: value?.minPhotos,
    hashtags: [...(value?.hashtags ?? [])],
  }
}

export function advertiserGuideToProto(guide: AdvertiserGuide): ProtoAdvertiserGuide {
  return create(AdvertiserGuideSchema, {
    text: guide.text,
    keywords: [...guide.keywords],
    minChars: guide.minChars,
    minPhotos: guide.minPhotos,
    hashtags: [...guide.hashtags],
  })
}
