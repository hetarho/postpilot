import { expect, it } from 'vitest'
import { moveInOrder, reorderTargetIndex } from './source-order'

// The drag and the two buttons mean the same thing, so the arithmetic under the
// drop is its own function (CLIP-55, CLIP-136).
it('drops a tile on the tile the pointer is over, and at the end past either edge', () => {
  const boxes = [
    { left: 0, right: 100 },
    { left: 100, right: 200 },
    { left: 200, right: 300 },
  ]
  expect(reorderTargetIndex(boxes, 50)).toBe(0)
  expect(reorderTargetIndex(boxes, 150)).toBe(1)
  expect(reorderTargetIndex(boxes, 250)).toBe(2)
  // Past the right edge lands last; before the first tile lands first.
  expect(reorderTargetIndex(boxes, 9000)).toBe(2)
  expect(reorderTargetIndex(boxes, -50)).toBe(0)
  expect(reorderTargetIndex([], 10)).toBe(0)
  // A scrolled strip is measured on the tiles themselves, so negative lefts
  // read exactly like positive ones.
  expect(
    reorderTargetIndex(
      [
        { left: -300, right: -200 },
        { left: -200, right: -100 },
      ],
      -150,
    ),
  ).toBe(1)
})

it('moves one entry and clamps a target outside the list', () => {
  const order = ['a', 'b', 'c']
  expect(moveInOrder(order, 0, 1)).toEqual(['b', 'a', 'c'])
  expect(moveInOrder(order, 2, 0)).toEqual(['c', 'a', 'b'])
  // "Earlier" on the first and "later" on the last change nothing.
  expect(moveInOrder(order, 0, -1)).toEqual(order)
  expect(moveInOrder(order, 2, 3)).toEqual(order)
  // The list handed in is never mutated.
  expect(order).toEqual(['a', 'b', 'c'])
})
