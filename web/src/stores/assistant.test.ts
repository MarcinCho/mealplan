import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAssistant } from './assistant'
import type { IngredientQuery } from '../api/types'

const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers })

function queries(n: number): IngredientQuery[] {
  return Array.from({ length: n }, (_, i) => ({ name: `s${i}` }))
}

// Echoes each ingredient back as a suggestion so order can be checked.
const echo = (init?: RequestInit) =>
  json({ suggestions: (JSON.parse(init?.body as string).ingredients as IngredientQuery[]).map((q) => ({ name: q.name, kind: 'none', reason: '' })) })

describe('useAssistant', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('matchIngredients splits 30 queries into batches of 25 and 5 and keeps the order', async () => {
    const spy = vi.fn(async (_u: string, init?: RequestInit) => echo(init))
    vi.stubGlobal('fetch', spy)
    const res = await useAssistant().matchIngredients(queries(30))
    expect(spy).toHaveBeenCalledTimes(2)
    expect(JSON.parse(spy.mock.calls[0][1]!.body as string).ingredients).toHaveLength(25)
    expect(JSON.parse(spy.mock.calls[1][1]!.body as string).ingredients).toHaveLength(5)
    expect(res.map((s) => s.name)).toEqual(queries(30).map((q) => q.name))
  })

  it('waits Retry-After on 429 and retries the same batch', async () => {
    vi.useFakeTimers()
    let n = 0
    const spy = vi.fn(async (_u: string, init?: RequestInit) =>
      n++ === 0 ? json({ code: 'rate_limited', message: 'slow' }, 429, { 'Retry-After': '7' }) : echo(init))
    vi.stubGlobal('fetch', spy)
    const p = useAssistant().matchIngredients(queries(2))
    await vi.advanceTimersByTimeAsync(6900)
    expect(spy).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(200)
    expect(await p).toHaveLength(2)
    expect(spy).toHaveBeenCalledTimes(2)
    expect(spy.mock.calls[1][1]!.body).toBe(spy.mock.calls[0][1]!.body)
  })

  it('falls back to 10 s without Retry-After and caps the wait at 60 s', async () => {
    vi.useFakeTimers()
    const answers = [
      () => json({ code: 'rate_limited', message: '' }, 429),
      () => json({ code: 'rate_limited', message: '' }, 429, { 'Retry-After': '600' }),
    ]
    const spy = vi.fn(async (_u: string, init?: RequestInit) => (answers.shift()?.() ?? echo(init)))
    vi.stubGlobal('fetch', spy)
    const p = useAssistant().matchIngredients(queries(1))
    await vi.advanceTimersByTimeAsync(9900)
    expect(spy).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(200)
    expect(spy).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(59700)
    expect(spy).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(400)
    expect(await p).toHaveLength(1)
  })

  it('exposes the seconds left while waiting out a 429', async () => {
    vi.useFakeTimers()
    let n = 0
    vi.stubGlobal('fetch', vi.fn(async (_u: string, init?: RequestInit) =>
      n++ === 0 ? json({ code: 'rate_limited', message: '' }, 429, { 'Retry-After': '3' }) : echo(init)))
    const a = useAssistant()
    expect(a.waitingSeconds).toBe(0)
    const p = a.matchIngredients(queries(1))
    await vi.advanceTimersByTimeAsync(10)
    expect(a.waitingSeconds).toBe(3)
    await vi.advanceTimersByTimeAsync(1000)
    expect(a.waitingSeconds).toBe(2)
    await vi.advanceTimersByTimeAsync(2100)
    await p
    expect(a.waitingSeconds).toBe(0)
  })

  it('does not retry other errors', async () => {
    const spy = vi.fn(async () => json({ code: 'assistant_bad_key', message: 'x' }, 502))
    vi.stubGlobal('fetch', spy)
    await expect(useAssistant().matchIngredients(queries(1))).rejects.toMatchObject({ code: 'assistant_bad_key' })
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('save omits the key when undefined and sends it when given', async () => {
    const spy = vi.fn(async () => json({ configured: true, keyHint: 'abcd', model: 'm', defaultModel: 'd' }))
    vi.stubGlobal('fetch', spy)
    const s = useAssistant()
    await s.save(undefined, 'm')
    await s.save('', 'm')
    expect(JSON.parse((spy.mock.calls[0] as unknown as [string, RequestInit])[1].body as string)).toEqual({ model: 'm' })
    expect(JSON.parse((spy.mock.calls[1] as unknown as [string, RequestInit])[1].body as string)).toEqual({ key: '', model: 'm' })
    expect(s.keyHint).toBe('abcd')
  })
})
