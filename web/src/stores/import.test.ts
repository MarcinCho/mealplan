import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useImport } from './import'

const sample = { title: 'Zupa', servings: 2, sourceUrl: '', tags: [], ingredients: [], steps: [], warnings: [] }

describe('useImport', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }

  it('posts a link and keeps the draft', async () => {
    const spy = mockFetch(200, sample)
    const s = useImport()
    const d = await s.fromUrl('https://example.com/z')
    expect(d.title).toBe('Zupa')
    expect(s.draft?.servings).toBe(2)
    const [url, init] = spy.mock.calls[0]
    expect(url).toBe('/api/v1/recipes/import/url')
    expect(JSON.parse(String(init?.body))).toEqual({ url: 'https://example.com/z' })
  })

  it('posts pasted text', async () => {
    const spy = mockFetch(200, sample)
    await useImport().fromText('Zupa\n1 l wody')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/recipes/import/text')
  })

  it('leaves no draft behind when the import fails and clear() resets everything', async () => {
    mockFetch(422, { code: 'import_unparsable', message: 'x' })
    const s = useImport()
    await expect(s.fromText('x')).rejects.toMatchObject({ code: 'import_unparsable' })
    expect(s.draft).toBeNull()
    mockFetch(200, sample)
    await s.fromText('x')
    s.targetId = 'r1'
    s.clear()
    expect(s.draft).toBeNull()
    expect(s.targetId).toBe('')
  })
})
