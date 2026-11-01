import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSession } from './session'
import { ApiError } from '../api/client'

const me = {
  user: { id: 'u1', login: 'marc', displayName: 'Marc' },
  household: { id: 'h1', name: 'Dom' },
  role: 'admin',
  csrfToken: 'csrf',
}

function json(status: number, body?: unknown) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), { status }))
}

function route(map: Record<string, () => Promise<Response>>) {
  return vi.fn((url: string, init?: RequestInit) => {
    const handler = map[`${init?.method ?? 'GET'} ${url}`]
    if (!handler) throw new Error(`unexpected request ${init?.method} ${url}`)
    return handler()
  })
}

beforeEach(() => setActivePinia(createPinia()))
afterEach(() => vi.unstubAllGlobals())

describe('session store', () => {
  it('bootstrap loads status and the current user', async () => {
    vi.stubGlobal('fetch', route({
      'GET /api/v1/auth/status': () => json(200, { setupRequired: false, mode: 'selfhost' }),
      'GET /api/v1/auth/me': () => json(200, me),
    }))
    const s = useSession()
    await s.bootstrap()
    expect(s.loaded).toBe(true)
    expect(s.me?.user.displayName).toBe('Marc')
    expect(s.isAdmin).toBe(true)
  })

  it('bootstrap treats 401 as logged out', async () => {
    vi.stubGlobal('fetch', route({
      'GET /api/v1/auth/status': () => json(200, { setupRequired: true, mode: 'selfhost' }),
      'GET /api/v1/auth/me': () => json(401, { code: 'unauthenticated', message: 'x' }),
    }))
    const s = useSession()
    await s.bootstrap()
    expect(s.me).toBeNull()
    expect(s.setupRequired).toBe(true)
  })

  it('login fetches the profile afterwards', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/login': () => json(204),
      'GET /api/v1/auth/me': () => json(200, me),
    }))
    const s = useSession()
    await s.login('marc', 'pw')
    expect(s.me?.household.name).toBe('Dom')
  })

  it('reports a dropped session cookie (Secure cookie over plain HTTP)', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/login': () => json(204),
      'GET /api/v1/auth/me': () => json(401, { code: 'unauthenticated', message: 'x' }),
    }))
    const s = useSession()
    const err = await s.login('marc', 'pw').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('cookie_not_stored')
  })

  it('setup clears setupRequired and logout clears the user', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/setup': () => json(204),
      'GET /api/v1/auth/me': () => json(200, me),
      'POST /api/v1/auth/logout': () => json(204),
    }))
    const s = useSession()
    s.setupRequired = true
    await s.setup({ householdName: 'Dom', login: 'marc', password: 'pw', displayName: 'Marc' })
    expect(s.setupRequired).toBe(false)
    expect(s.me).not.toBeNull()
    await s.logout()
    expect(s.me).toBeNull()
  })

  it('setup that succeeded but lost the cookie still clears setupRequired', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/setup': () => json(204),
      'GET /api/v1/auth/me': () => json(401, { code: 'unauthenticated', message: 'x' }),
    }))
    const s = useSession()
    s.setupRequired = true
    const err = await s.setup({ householdName: 'Dom', login: 'marc', password: 'pw', displayName: 'Marc' }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('cookie_not_stored')
    expect(s.setupRequired).toBe(false)
  })

  it('setup_done error clears setupRequired', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/setup': () => json(409, { code: 'setup_done', message: 'x' }),
    }))
    const s = useSession()
    s.setupRequired = true
    const err = await s.setup({ householdName: 'Dom', login: 'marc', password: 'pw', displayName: 'Marc' }).catch((e) => e)
    expect(err.code).toBe('setup_done')
    expect(s.setupRequired).toBe(false)
  })

  it('other setup errors keep setupRequired', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/setup': () => json(400, { code: 'invalid_input', message: 'x' }),
    }))
    const s = useSession()
    s.setupRequired = true
    await s.setup({ householdName: 'Dom', login: 'marc', password: 'pw', displayName: 'Marc' }).catch(() => {})
    expect(s.setupRequired).toBe(true)
  })

  it('logout clears local state even when the request fails', async () => {
    vi.stubGlobal('fetch', route({
      'POST /api/v1/auth/logout': () => json(500, { code: 'internal', message: 'x' }),
    }))
    const s = useSession()
    s.me = me as never
    await s.logout().catch(() => {})
    expect(s.me).toBeNull()
  })
})
