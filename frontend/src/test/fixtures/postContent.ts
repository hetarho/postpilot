import { create } from '@bufbuild/protobuf'
import { BlockSchema, BlockType, ObservationSchema, PostContentSchema } from '@/shared/api'

export const POST_CONTENT_FIXTURE = create(PostContentSchema, {
  title: '비 온 뒤의 제주',
  summary: '비가 그친 뒤 천천히 걸은 하루',
  tags: ['제주', '산책', '여행'],
  blocks: [
    create(BlockSchema, { type: BlockType.TEXT, content: '비가 그치기를 기다렸다.' }),
    create(BlockSchema, { type: BlockType.HEADING, level: 2, content: '바닷가로' }),
    create(BlockSchema, {
      type: BlockType.IMAGE,
      file: 'IMG_1.jpg',
      alt: '잔잔한 제주 바다',
      caption: '비 뒤의 바다',
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
})
