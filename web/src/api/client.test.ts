import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, setCsrfToken } from './client'

function reply(status: number, body?: unknown, headers: Record<string, string> = {}) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status, headers }))
}

afterEach(() => {
  vi.unstubAllGlobals()
  setCsrfToken('')
})

describe('api client', () => {
  it('sends JSON bodies and the CSRF header on writes only', async () => {
    const fetchMock = vi.fn(() => reply(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    setCsrfToken('tok')
    await api('POST', '/auth/invites', { role: 'member' })
    await api('GET', '/auth/me')
    const [, post] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    const [, get] = fetchMock.mock.calls[1] as unknown as [string, RequestInit]
    expect((post.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok')
    expect((post.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(post.body).toBe('{"role":"member"}')
    expect((get.headers as Record<string, string>)['X-CSRF-Token']).toBeUndefined()
  })

  it('returns undefined for 204', async () => {
    vi.stubGlobal('fetch', vi.fn(() => reply(204)))
    expect(await api('POST', '/auth/logout')).toBeUndefined()
  })

  it('throws ApiError with code, message and Retry-After', async () => {
    vi.stubGlobal('fetch', vi.fn(() => reply(429, { code: 'rate_limited', message: 'slow down' }, { 'Retry-After': '7' })))
    await expect(api('POST', '/auth/login', {})).rejects.toMatchObject({
      status: 429, code: 'rate_limited', message: 'slow down', retryAfter: 7,
    })
  })

  it('wraps non-JSON failures', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('<html>', { status: 502 }))))
    const err = await api('GET', '/auth/me').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(502)
  })
})
