import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { toTistory } from './convert'

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
