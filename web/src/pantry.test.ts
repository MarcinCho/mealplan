import { describe, expect, it } from 'vitest'
import { formatStock, isExpired, reviewDue, stepFor } from './pantry'
import type { PantryItem } from './api/types'

const item = (o: Partial<PantryItem>): PantryItem => ({
  id: '1', productId: 'p', name: 'Ryż', quantity: 500, unit: 'g', expiresOn: '', staple: false, ...o,
})
const now = new Date(2026, 10, 8, 12)
const daysAgo = (n: number) => new Date(now.getTime() - n * 86400000).toISOString()

describe('reviewDue', () => {
  it('is false without items', () => expect(reviewDue(null, 0, now)).toBe(false))
  it('is true when never reviewed', () => expect(reviewDue(null, 3, now)).toBe(true))
  it('is false after 29 days', () => expect(reviewDue(daysAgo(29), 3, now)).toBe(false))
  it('is true after 31 days', () => expect(reviewDue(daysAgo(31), 3, now)).toBe(true))
})

describe('stepFor', () => {
  it('uses 50 for g and ml, 1 otherwise', () => {
    expect(stepFor('g')).toBe(50)
    expect(stepFor('ml')).toBe(50)
    expect(stepFor('szt')).toBe(1)
    expect(stepFor('puszka')).toBe(1)
  })
})

describe('formatStock', () => {
  it('formats plain and large amounts', () => {
    expect(formatStock(item({}))).toBe('500 g')
    expect(formatStock(item({ quantity: 1500 }))).toBe('1,5 kg')
    expect(formatStock(item({ quantity: 2000, unit: 'ml' }))).toBe('2 l')
    expect(formatStock(item({ quantity: 3, unit: 'szt' }))).toBe('3 szt')
  })
  it('shows staples', () => {
    expect(formatStock(item({ quantity: null, staple: true }))).toBe('stały zapas')
  })
})

describe('isExpired', () => {
  it('compares with the local date', () => {
    expect(isExpired('2026-11-07', now)).toBe(true)
    expect(isExpired('2026-11-08', now)).toBe(false)
    expect(isExpired('', now)).toBe(false)
  })
})

import { afterEach, beforeEach, vi } from 'vitest'
import { createSerialSaver, validExpiry } from './pantry'

describe('validExpiry', () => {
  it('accepts empty and real dates from 1900', () => {
    expect(validExpiry('')).toBe(true)
    expect(validExpiry('2026-11-08')).toBe(true)
    expect(validExpiry('0002-01-01')).toBe(false)
    expect(validExpiry('1899-12-31')).toBe(false)
    expect(validExpiry('2026-02-30')).toBe(false)
    expect(validExpiry('2026-1-1')).toBe(false)
  })
})

describe('createSerialSaver', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('sends one value for three quick taps', async () => {
    const send = vi.fn(async (_v: number) => {})
    const s = createSerialSaver<number>({ send, delay: 400, onError: vi.fn() })
    s.set(550); s.set(600); s.set(650)
    expect(send).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(400)
    expect(send).toHaveBeenCalledTimes(1)
    expect(send).toHaveBeenCalledWith(650)
    expect(s.pending).toBe(false)
  })

  it('waits for a slow in-flight send, then sends the latest value', async () => {
    let release!: () => void
    const calls: number[] = []
    const send = vi.fn((v: number) => {
      calls.push(v)
      return calls.length === 1 ? new Promise<void>((r) => (release = r)) : Promise.resolve()
    })
    const idle = vi.fn()
    const s = createSerialSaver<number>({ send, delay: 400, onError: vi.fn(), onIdle: idle })
    s.set(550)
    await vi.advanceTimersByTimeAsync(400)
    s.set(600)
    await vi.advanceTimersByTimeAsync(400)
    expect(calls).toEqual([550]) // still waiting for the first
    expect(s.pending).toBe(true)
    release()
    await vi.advanceTimersByTimeAsync(0)
    expect(calls).toEqual([550, 600])
    expect(idle).toHaveBeenCalledTimes(1)
    expect(s.pending).toBe(false)
  })

  it('reports a failure once and drops queued changes', async () => {
    const err = new Error('x')
    const send = vi.fn(async (_v: number) => { throw err })
    const onError = vi.fn()
    const s = createSerialSaver<number>({ send, delay: 400, onError })
    s.set(1)
    await vi.advanceTimersByTimeAsync(400)
    expect(onError).toHaveBeenCalledWith(err)
    expect(s.pending).toBe(false)
    await vi.advanceTimersByTimeAsync(1000)
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('merges patches', async () => {
    const send = vi.fn(async (_v: object) => {})
    const s = createSerialSaver<object>({ send, delay: 10, onError: vi.fn(), merge: (a, b) => ({ ...a, ...b }) })
    s.set({ quantity: 5 }); s.set({ expiresOn: '2027-01-01' })
    await vi.advanceTimersByTimeAsync(10)
    expect(send).toHaveBeenCalledWith({ quantity: 5, expiresOn: '2027-01-01' })
  })

  it('cancel drops queued changes and silences an in-flight send', async () => {
    let fail!: (e: unknown) => void
    const send = vi.fn((_v: number) => new Promise<void>((_, rej) => (fail = rej)))
    const onError = vi.fn()
    const onIdle = vi.fn()
    const s = createSerialSaver<number>({ send, delay: 100, onError, onIdle })
    s.set(1)
    await vi.advanceTimersByTimeAsync(100)
    s.set(2)
    s.cancel()
    fail(new Error('404'))
    await vi.advanceTimersByTimeAsync(1000)
    expect(send).toHaveBeenCalledTimes(1)
    expect(onError).not.toHaveBeenCalled()
    expect(onIdle).not.toHaveBeenCalled()
    expect(s.pending).toBe(false)
  })

  it('merges a staple toggle with a pending quantity', async () => {
    const send = vi.fn(async (_v: object) => {})
    const s = createSerialSaver<object>({ send, delay: 10, onError: vi.fn(), merge: (a, b) => ({ ...a, ...b }) })
    s.set({ quantity: 650 }); s.set({ staple: true })
    await vi.advanceTimersByTimeAsync(10)
    expect(send).toHaveBeenCalledWith({ quantity: 650, staple: true })
  })
})
