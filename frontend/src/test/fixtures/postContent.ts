import { create } from '@bufbuild/protobuf'
import type { PostImage } from '@/entities/image'
import { BlockSchema, BlockType, ObservationSchema, PostContentSchema } from '@/shared/api'

export const POST_IMAGES_FIXTURE: PostImage[] = [
  {
    id: 'image-1',
    filename: 'IMG_1.jpg',
    width: 1024,
    height: 768,
    bytes: 200_000,
    viewUrl: 'https://api.postpilot.test/private/IMG_1.jpg?signature=temporary',
  },
  {
    id: 'image-2',
    filename: 'IMG_2.jpg',
    width: 768,
    height: 1024,
    bytes: 180_000,
    viewUrl: 'https://bucket.r2.cloudflarestorage.com/private/IMG_2.jpg?signature=temporary',
  },
]

export const POST_CONTENT_FIXTURE = create(PostContentSchema, {
  title: '비 온 뒤의 제주',
  summary: '비가 그친 뒤 천천히 걸은 하루',
  tags: ['제주', '산책', '여행'],
  blocks: [
    create(BlockSchema, { type: BlockType.TEXT, content: '비가 그치기를 기다렸다.' }),
    create(BlockSchema, {
      type: BlockType.TEXT,
      content: '<바다> & "바람"도 잠잠했다.',
    }),
    create(BlockSchema, { type: BlockType.HEADING, level: 2, content: '바닷가로' }),
    create(BlockSchema, {
      type: BlockType.IMAGE,
      file: 'IMG_1.jpg',
      alt: '잔잔한 제주 바다',
      caption: '비 뒤의 바다',
    }),
    create(BlockSchema, { type: BlockType.HEADING, level: 3, content: '챙긴 것' }),
    create(BlockSchema, {
      type: BlockType.IMAGE,
      file: 'IMG_2.jpg',
      alt: '구름 사이 햇빛',
    }),
    create(BlockSchema, { type: BlockType.QUOTE, content: '서두르지 않아도 괜찮다.' }),
    create(BlockSchema, { type: BlockType.LIST, items: ['우산', '따뜻한 차'] }),
  ],
})

export const OBSERVATION_FIXTURE = create(ObservationSchema, {
  file: 'IMG_1.jpg',
  scene: '비가 그친 바닷가',
  mood: '차분함',
  visibleText: 'JEJU',
  objects: ['우산', '파도'],
  model: 'openrouter/observer',
})

/** A complete snapshot over POST_IMAGES_FIXTURE — what a post that has already been observed
 *  once looks like, so a start on it goes through the re-observation picker. */
export const OBSERVATIONS_FIXTURE = [
  OBSERVATION_FIXTURE,
  create(ObservationSchema, {
    file: 'IMG_2.jpg',
    scene: '구름 사이 햇빛',
    mood: '맑음',
    model: 'openrouter/observer',
  }),
]
