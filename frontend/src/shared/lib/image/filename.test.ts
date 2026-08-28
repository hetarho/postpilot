import { describe, expect, it } from 'vitest'
import { dedupeFilename, jpegFilename, splitFilename } from './filename'

describe('splitFilename', () => {
  it('splits at the last dot and lower-cases the extension', () => {
    expect(splitFilename('IMG_1.HEIC')).toEqual({ stem: 'IMG_1', extension: 'heic' })
    expect(splitFilename('a.b.jpg')).toEqual({ stem: 'a.b', extension: 'jpg' })
    expect(splitFilename('noext')).toEqual({ stem: 'noext', extension: '' })
    expect(splitFilename('.jpg')).toEqual({ stem: '', extension: 'jpg' })
    expect(splitFilename('dir/x.png')).toEqual({ stem: 'x', extension: 'png' })
  })
})

describe('jpegFilename', () => {
  it('keeps the stem and swaps the extension for .jpg', () => {
    expect(jpegFilename('IMG_1.HEIC')).toBe('IMG_1.jpg')
    expect(jpegFilename('trip photo.png')).toBe('trip photo.jpg')
  })

  it('handles a name with no extension or only a path', () => {
    expect(jpegFilename('photo')).toBe('photo.jpg')
    expect(jpegFilename('dir/IMG_2.jpeg')).toBe('IMG_2.jpg')
    expect(jpegFilename('   .jpg')).toBe('photo.jpg')
  })
})

describe('dedupeFilename', () => {
  it('leaves a free name alone', () => {
    expect(dedupeFilename('IMG_1.jpg', ['IMG_2.jpg'])).toBe('IMG_1.jpg')
  })

  it('appends a serial when the name is taken, skipping serials that are taken too', () => {
    expect(dedupeFilename('IMG_1.jpg', ['IMG_1.jpg'])).toBe('IMG_1 (2).jpg')
    expect(dedupeFilename('IMG_1.jpg', ['IMG_1.jpg', 'IMG_1 (2).jpg'])).toBe('IMG_1 (3).jpg')
  })
})
