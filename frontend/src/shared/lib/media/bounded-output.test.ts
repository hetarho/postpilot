import { expect, it } from 'vitest'
import { createBoundedMediaOutput } from './bounded-output'
const bytesOf = (file: Blob) =>
  new Promise<ArrayBuffer>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(reader.error)
    reader.readAsArrayBuffer(file)
  })

it('applies backward, overlapping and sparse writes without reallocating the whole output', async () => {
  const output = await createBoundedMediaOutput(
    { maxBytes: 16, pageBytes: 4 },
    new AbortController().signal,
  )
  const writer = output.stream.getWriter()
  const write = (position: number, data: number[]) =>
    writer.write({ type: 'write', position, data: new Uint8Array(data) })
  await write(8, [8, 9, 10, 11])
  await write(0, [1, 2, 3, 4, 5])
  await write(3, [13, 14, 15])
  expect(output.measurements()).toEqual({ bytes: 12, allocatedBytes: 12, writes: 3 })
  await writer.close()
  const file = await output.file()
  expect([...new Uint8Array(await bytesOf(file))]).toEqual([
    1, 2, 3, 13, 14, 15, 0, 0, 8, 9, 10, 11,
  ])
  expect(await output.file()).toBe(file)
  expect(output.measurements().allocatedBytes).toBe(0)
  await output.dispose()
  await expect(output.file()).rejects.toThrow('MEDIA_OUTPUT_INCOMPLETE')
})
it('allows the exact ceiling and refuses a one-byte overflow before allocation or publication', async () => {
  const output = await createBoundedMediaOutput(
    { maxBytes: 8, pageBytes: 4 },
    new AbortController().signal,
  )
  const writer = output.stream.getWriter()
  await writer.write({ type: 'write', position: 7, data: new Uint8Array([7]) })
  expect(output.measurements().allocatedBytes).toBe(4)
  await expect(
    writer.write({ type: 'write', position: 8, data: new Uint8Array([8]) }),
  ).rejects.toThrow('MEDIA_OUTPUT_SIZE_LIMIT')
  await expect(output.file()).rejects.toThrow('MEDIA_OUTPUT_SIZE_LIMIT')
  expect(output.measurements()).toEqual({ bytes: 8, allocatedBytes: 4, writes: 1 })
  await output.dispose()
})
it('releases memory and refuses a late queued write after cancellation', async () => {
  const controller = new AbortController()
  const output = await createBoundedMediaOutput({ maxBytes: 8, pageBytes: 4 }, controller.signal)
  const writer = output.stream.getWriter()
  await writer.write({ type: 'write', position: 0, data: new Uint8Array([1, 2]) })
  controller.abort(new Error('left page'))
  await expect(
    writer.write({ type: 'write', position: 0, data: new Uint8Array([3]) }),
  ).rejects.toThrow('left page')
  expect(output.measurements().allocatedBytes).toBe(0)
})
