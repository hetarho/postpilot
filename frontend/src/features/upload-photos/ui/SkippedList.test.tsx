import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UploadItem } from '../model/upload-batch'
import { SkippedList } from './SkippedList'

describe('SkippedList', () => {
  it('keeps the dismiss target available beside a long filename', async () => {
    const user = userEvent.setup()
    const onDismiss = vi.fn()
    const name = `${'긴-파일명-'.repeat(20)}.heic`
    const item: UploadItem = {
      id: 'skipped-1',
      name,
      filename: name,
      status: 'skipped',
      reason: 'heif-unsupported',
    }
    render(<SkippedList items={[item]} onDismiss={onDismiss} />)

    expect(screen.getByText(name)).toHaveClass('min-w-0', 'flex-1', 'truncate')
    const dismiss = screen.getByRole('button', { name: `${name} 목록에서 지우기` })
    expect(dismiss).toHaveClass('size-11', 'shrink-0')
    await user.click(dismiss)
    expect(onDismiss).toHaveBeenCalledWith('skipped-1')
  })
})
