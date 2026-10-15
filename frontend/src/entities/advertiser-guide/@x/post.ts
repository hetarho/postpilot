// What the post entity may import from the advertiser-guide entity (FSD cross-import, `@x`): a
// post carries its guide (POST-118), so the post's model needs the guide's shape and mappers.
export type { AdvertiserGuide } from '../model/types'
export { advertiserGuideFromProto, advertiserGuideToProto } from '../api/mappers'
