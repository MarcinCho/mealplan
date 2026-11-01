import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyText } from './clipboard'

afterEach(() => vi.unstubAllGlobals())

describe('copyText', () => {
  it('returns false when the clipboard API is missing (insecure context)', async () => {
    vi.stubGlobal('navigator', {})
    expect(await copyText('x')).toBe(false)
  })
  it('returns false when writeText rejects', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn(() => Promise.reject(new Error('denied'))) } })
    expect(await copyText('x')).toBe(false)
  })
  it('returns true on success', async () => {
    const writeText = vi.fn(() => Promise.resolve())
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    expect(await copyText('x')).toBe(true)
    expect(writeText).toHaveBeenCalledWith('x')
  })
})
