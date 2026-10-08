import { expect, it } from 'vitest'
import { readBuilderMetadata } from './builder-metadata'

it.each([undefined, '', 'null', '[]', '[1,2]', 'true', '42', '"text"', '{'])(
  'recovers canonical editing when private metadata is %s',
  (raw) => expect(readBuilderMetadata(raw)).toEqual({}),
)

it('retains independent builder state and unknown private fields in valid object metadata', () => {
  const metadata = {
    numberMemory: { tagCount: '8' },
    body: { source: '<write>Body</write>' },
    future: { version: 2 },
  }
  expect(readBuilderMetadata(JSON.stringify(metadata))).toEqual(metadata)
})
