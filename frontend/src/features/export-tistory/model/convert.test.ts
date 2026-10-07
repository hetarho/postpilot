import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import {
  POST_CONTENT_WITH_GROUPS_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  POST_IMAGES_FIXTURE,
  POST_CONTENT_FIXTURE,
} from '@/test/fixtures/postContent'
import { toTistory } from './convert'
import {
  OWNER_CONTROL_CONTENT,
  OWNER_CONTROL_TEXT,
  PRIVATE_REVIEW_SENTINEL,
  PRIVATE_REQUEST_SENTINEL,
} from '@/test/fixtures/ownerControlContent'

it('preserves genuine marker-like owner writing while excluding origin and technical sidecars', () => {
  for (const language of ['ko', 'en'] as const) {
    const output = toTistory(OWNER_CONTROL_CONTENT, POST_IMAGES_FIXTURE, language)
    const parsed = new DOMParser().parseFromString(output, 'text/html')
    expect([...parsed.querySelectorAll('p')].map((node) => node.textContent)).toContain(
      OWNER_CONTROL_TEXT,
    )
    expect(parsed.querySelector('write')).toBeNull()
    expect(output).not.toContain(PRIVATE_REVIEW_SENTINEL)
    expect(output).not.toContain(PRIVATE_REQUEST_SENTINEL)
    expect(output.match(/text-origin-owner-foreground/g)).toHaveLength(1)
    expect(OWNER_CONTROL_CONTENT.blocks[0]!.content).toBe(OWNER_CONTROL_TEXT)
  }
})

it('converts every block to the Tistory fragment contract', () => {
  const output = toTistory(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')
  const doc = new DOMParser().parseFromString(`<div id="root">${output}</div>`, 'text/html')
  const root = doc.querySelector('#root')

  expect(output).toMatchSnapshot()
  expect(doc.querySelector('parsererror')).toBeNull()
  expect(output).not.toMatch(/<\/?(?:html|body)\b/i)
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
  expect(root).not.toBeNull()
  for (const image of root?.querySelectorAll('img') ?? []) {
    expect(image.getAttribute('src')).toBe('')
    expect(image.nextSibling?.nodeType).toBe(Node.COMMENT_NODE)
    expect(image.nextSibling?.textContent).toContain(image.dataset.file)
  }
})

it('keeps a comment-closing filename inside the adjacent comment', () => {
  const filename = 'photo--> <script>bad</script>.jpg'
  const content = create(PostContentSchema, {
    blocks: [create(BlockSchema, { type: BlockType.IMAGE, file: filename, alt: '사진' })],
  })
  const output = toTistory(content, [], 'ko')
  const doc = new DOMParser().parseFromString(`<div id="root">${output}</div>`, 'text/html')
  const image = doc.querySelector('img')

  expect(image?.dataset.file).toBe(filename)
  expect(image?.nextSibling?.nodeType).toBe(Node.COMMENT_NODE)
  expect(doc.querySelector('script')).toBeNull()
})

it('uses the content provenance for app-owned English upload instructions', () => {
  const output = toTistory(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'en')

  expect(output).toContain('replace src after uploading')
  expect(output).not.toContain('업로드 후 src 교체')
})

// The same empty-source-plus-instruction shape an image takes: the editor has no URL to put
// here, and the author attaches the original by hand (VIDEO-14).
it('writes a video as an empty element with the replacement instruction', () => {
  const output = toTistory(POST_CONTENT_WITH_VIDEO_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  expect(output).toMatchSnapshot()
  expect(output).toContain('<video controls data-file="clip.mp4"></video>')
  expect(output).toContain('<!-- clip.mp4 업로드 후 영상 첨부 -->')
  expect(output).not.toContain('src="https')
})

// EXPORT-26: one figure naming its layout, each photo an empty-src image with its own replacement
// comment, and the group's one caption.
it('writes a photo group as one figure with its layout, its photos and one caption', () => {
  const output = toTistory(POST_CONTENT_WITH_GROUPS_FIXTURE, [], 'ko')
  expect(output).toMatchSnapshot()
  expect(output).toContain(
    '<figure data-layout="collage"><img src="" alt="창가" data-file="IMG_2.jpg"><!-- IMG_2.jpg 업로드 후 src 교체 --><img src="" alt="창가" data-file="IMG_3.jpg"><!-- IMG_3.jpg 업로드 후 src 교체 --><figcaption>창가 자리(2층)</figcaption></figure>',
  )
  expect(output).toContain('<figure data-layout="slide"><img src="" alt="" data-file="IMG_4.jpg">')
  expect(output.match(/<figcaption>/g)).toHaveLength(2)
})
