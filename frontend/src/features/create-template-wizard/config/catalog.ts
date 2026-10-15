/** What the wizard offers (TMPL-72): a code-owned tree of targets, each leaf an ordered list of
 *  section roles. The words are in this slice's i18n under the same keys; nothing here is a row,
 *  a config value or model output. */

/** What stands in a section: words the AI writes about the role, a 데이터 받기 field the post's
 *  author fills in (TMPL-43), or text fixed by the template. */
export type SectionContent = 'write' | 'ask' | 'literal'

export interface RoleDefault {
  content: SectionContent
  /** Photos placed together after the section's words; 0 is none. */
  photos: number
  /** Present on a role that may repeat per photo group (TMPL-18), and whether it does at first. */
  repeat?: boolean
}

const write = (photos: number, repeat?: boolean): RoleDefault =>
  repeat === undefined ? { content: 'write', photos } : { content: 'write', photos, repeat }
const ask: RoleDefault = { content: 'ask', photos: 0 }

export const ROLES = {
  // 맛집
  visitReason: write(0),
  locationParking: write(1),
  exterior: write(1),
  interiorMood: write(2),
  menuBoard: write(1),
  orderedMenu: write(1, true),
  service: write(0),
  priceValue: write(0),
  drawbacks: write(0),
  verdict: write(0),
  storeInfo: ask,
  courseFlow: write(1, true),
  reservation: write(0),
  drinksSnacks: write(1, false),
  orderMethod: write(1),
  packaging: write(1),
  deliveryTime: write(0),
  // 카페
  spaceSeats: write(2),
  orderedDrinks: write(1, true),
  goodToStay: write(0),
  price: write(0),
  takeoutGift: write(1),
  waitingReservation: write(0),
  photoSpot: write(2),
  // 뷰티
  locationBooking: write(0),
  ambienceHygiene: write(1),
  consultation: write(0),
  procedure: write(1, false),
  productsUsed: write(1),
  result: write(2),
  careTips: write(0),
  designChoice: write(1),
  skinDiagnosis: write(0),
  // 숙소
  bookingReason: write(0),
  locationTransport: write(1),
  checkIn: write(1),
  room: write(2),
  bathAmenity: write(1),
  facilities: write(2),
  meals: write(1),
  nearby: write(1),
  priceTips: write(0),
  stayInfo: ask,
  siteTent: write(2),
  toiletSink: write(1),
  // 체험·여가
  participationReason: write(0),
  space: write(1),
  progress: write(2),
  highlight: write(2),
  goodWith: write(0),
  priceDuration: write(0),
  suppliesTips: write(0),
  activityInfo: ask,
  // 제품
  receivedReason: write(0),
  components: write(2, false),
  designFirst: write(1),
  mainFeatures: write(1),
  howToUse: write(1),
  usageReview: write(1),
  pros: write(0),
  recommendedFor: write(0),
  priceWhere: ask,
  usagePeriodEnv: write(0),
  comparedWith: write(1),
  comparisonCriteria: write(0),
  comparisonResult: write(1),
  installNoise: write(1),
  textureSpread: write(1),
  tasteRecipe: write(1),
  safetyReaction: write(0),
} as const satisfies Record<string, RoleDefault>

export type RoleKey = keyof typeof ROLES

/** One target: a branch asks the next question among its children, a leaf lists its roles. */
export interface TargetNode {
  key: string
  children?: readonly TargetNode[]
  roles?: readonly RoleKey[]
}

const leaf = (key: string, roles: readonly RoleKey[]): TargetNode => ({ key, roles })

/** `roles` with `extra` placed after `after`, or at the end when `after` is absent. */
function insertAfter(roles: readonly RoleKey[], after: RoleKey, extra: readonly RoleKey[]) {
  const at = roles.indexOf(after)
  return at < 0
    ? [...roles, ...extra]
    : [...roles.slice(0, at + 1), ...extra, ...roles.slice(at + 1)]
}

function replace(roles: readonly RoleKey[], gone: readonly RoleKey[], instead: readonly RoleKey[]) {
  const at = roles.findIndex((role) => gone.includes(role))
  const kept = roles.filter((role) => !gone.includes(role))
  return [...kept.slice(0, at), ...instead, ...kept.slice(at)]
}

const RESTAURANT: readonly RoleKey[] = [
  'visitReason',
  'locationParking',
  'exterior',
  'interiorMood',
  'menuBoard',
  'orderedMenu',
  'service',
  'priceValue',
  'drawbacks',
  'verdict',
  'storeInfo',
]

const CAFE: readonly RoleKey[] = [
  'visitReason',
  'locationParking',
  'exterior',
  'spaceSeats',
  'menuBoard',
  'orderedDrinks',
  'goodToStay',
  'price',
  'drawbacks',
  'verdict',
  'storeInfo',
]

const BEAUTY: readonly RoleKey[] = [
  'visitReason',
  'locationBooking',
  'ambienceHygiene',
  'consultation',
  'procedure',
  'productsUsed',
  'result',
  'careTips',
  'price',
  'drawbacks',
  'verdict',
  'storeInfo',
]

const STAY: readonly RoleKey[] = [
  'bookingReason',
  'locationTransport',
  'checkIn',
  'room',
  'bathAmenity',
  'facilities',
  'meals',
  'nearby',
  'priceTips',
  'drawbacks',
  'verdict',
  'stayInfo',
]

const ACTIVITY: readonly RoleKey[] = [
  'participationReason',
  'locationBooking',
  'space',
  'progress',
  'highlight',
  'goodWith',
  'priceDuration',
  'suppliesTips',
  'drawbacks',
  'verdict',
  'activityInfo',
]

const PRODUCT: readonly RoleKey[] = [
  'receivedReason',
  'components',
  'designFirst',
  'mainFeatures',
  'howToUse',
  'usageReview',
  'pros',
  'drawbacks',
  'recommendedFor',
  'priceWhere',
]

/** The kind's own section beside the main features: 가전 설치·소음, 화장품 제형·발림성, 식품
 *  맛·조리법, 육아·반려 안전·반응; 생활·주방 needs none of its own. */
const PRODUCT_KIND_ROLE: Readonly<Record<string, RoleKey | undefined>> = {
  appliance: 'installNoise',
  cosmetics: 'textureSpread',
  food: 'tasteRecipe',
  living: undefined,
  babyPet: 'safetyReaction',
}

/** The review form: 언박싱·첫인상 drops 사용 후기 and leads with 구성품, 실사용 후기 adds 사용
 *  기간·환경, 비교 후기 adds what it is compared with, by what and with what result. */
function productRoles(kind: string, form: string): RoleKey[] {
  const own = PRODUCT_KIND_ROLE[kind]
  let roles: RoleKey[] = own ? insertAfter(PRODUCT, 'mainFeatures', [own]) : [...PRODUCT]
  if (form === 'unboxing')
    roles = [
      'components',
      ...roles.filter((role) => role !== 'components' && role !== 'usageReview'),
    ]
  if (form === 'longTerm') roles = insertAfter(roles, 'howToUse', ['usagePeriodEnv'])
  if (form === 'comparison')
    roles = insertAfter(roles, 'usageReview', [
      'comparedWith',
      'comparisonCriteria',
      'comparisonResult',
    ])
  return roles
}

const PRODUCT_FORMS = ['unboxing', 'longTerm', 'comparison'] as const

/** The v1 tree. Medical treatment and cosmetic procedure reviews are not a target: Korean medical
 *  advertising rules restrict treatment-experience testimonials. */
export const TARGET_TREE: TargetNode = {
  key: 'root',
  children: [
    {
      key: 'restaurant',
      children: [
        leaf('meal', RESTAURANT),
        leaf(
          'course',
          insertAfter(replace(RESTAURANT, ['orderedMenu'], ['courseFlow']), 'visitReason', [
            'reservation',
          ]),
        ),
        leaf('bar', insertAfter(RESTAURANT, 'orderedMenu', ['drinksSnacks'])),
        leaf(
          'takeout',
          replace(
            RESTAURANT,
            ['exterior', 'interiorMood', 'service'],
            ['orderMethod', 'packaging', 'deliveryTime'],
          ),
        ),
      ],
    },
    {
      key: 'cafe',
      children: [
        leaf('dessertBakery', insertAfter(CAFE, 'orderedDrinks', ['takeoutGift'])),
        leaf('brunch', insertAfter(CAFE, 'locationParking', ['waitingReservation'])),
        leaf('spaceView', insertAfter(CAFE, 'spaceSeats', ['photoSpot'])),
      ],
    },
    {
      key: 'beauty',
      children: [
        leaf('hair', BEAUTY),
        leaf('nailLash', insertAfter(BEAUTY, 'consultation', ['designChoice'])),
        leaf('skinEsthetic', insertAfter(BEAUTY, 'consultation', ['skinDiagnosis'])),
      ],
    },
    {
      key: 'stay',
      children: [
        leaf('hotelResort', STAY),
        leaf('pensionPool', STAY),
        leaf('camping', replace(STAY, ['room', 'bathAmenity'], ['siteTent', 'toiletSink'])),
      ],
    },
    {
      key: 'activity',
      children: [
        leaf('oneDayClass', ACTIVITY),
        leaf('leisure', ACTIVITY),
        leaf('exhibition', ACTIVITY),
      ],
    },
    {
      key: 'product',
      children: Object.keys(PRODUCT_KIND_ROLE).map((kind) => ({
        key: kind,
        children: PRODUCT_FORMS.map((form) => leaf(form, productRoles(kind, form))),
      })),
    },
  ],
}

/** The node a path of choices leads to, or undefined for a path the tree does not hold. */
export function nodeAt(
  path: readonly string[],
  tree: TargetNode = TARGET_TREE,
): TargetNode | undefined {
  let node: TargetNode | undefined = tree
  for (const key of path) node = node?.children?.find((child) => child.key === key)
  return node
}

/** Every leaf with the path that reaches it, for the catalog's own checks. */
export function leaves(tree: TargetNode = TARGET_TREE, path: readonly string[] = []) {
  const out: Array<{ path: string[]; roles: readonly RoleKey[] }> = []
  for (const child of tree.children ?? []) {
    const next = [...path, child.key]
    if (child.roles) out.push({ path: next, roles: child.roles })
    else out.push(...leaves(child, next))
  }
  return out
}

/** Every branch's key, which names the question it asks. */
export function branches(tree: TargetNode = TARGET_TREE): string[] {
  if (!tree.children) return []
  return [tree.key, ...tree.children.flatMap((child) => branches(child))]
}
