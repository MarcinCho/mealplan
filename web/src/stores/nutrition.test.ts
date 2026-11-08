import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNutrition } from './nutrition'

describe('useNutrition', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  function mockFetch(status: number, body: unknown) {
    const spy = vi.fn(async (_url: string, _init?: RequestInit) => new Response(status === 204 ? null : JSON.stringify(body), { status }))
    vi.stubGlobal('fetch', spy)
    return spy
  }
  const goal = { kcal: 2000, protein: null, carbs: null, fat: null }

  it('loads settings', async () => {
    const spy = mockFetch(200, { enabled: true, goal })
    const s = useNutrition()
    await s.loadSettings()
    expect(s.settings).toEqual({ enabled: true, goal })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/me/nutrition')
  })

  it('saves settings with PUT and stores the saved body', async () => {
    const spy = mockFetch(200, { enabled: true, goal })
    const s = useNutrition()
    await s.saveSettings({ enabled: true, goal })
    expect(spy.mock.calls[0][1]!.method).toBe('PUT')
    expect(JSON.parse(String(spy.mock.calls[0][1]!.body))).toEqual({ enabled: true, goal })
    expect(s.settings?.enabled).toBe(true)
  })

  it('sends emptied goal fields as null', async () => {
    const spy = mockFetch(200, { enabled: true, goal })
    await useNutrition().saveSettings({ enabled: true, goal: { kcal: '' as unknown as number, protein: 90, carbs: null, fat: null } })
    expect(JSON.parse(String(spy.mock.calls[0][1]!.body)).goal).toEqual({ kcal: null, protein: 90, carbs: null, fat: null })
  })

  it('loads days into a map by date', async () => {
    const spy = mockFetch(200, { enabled: true, days: [{ date: '2026-11-08' }, { date: '2026-11-09' }] })
    const s = useNutrition()
    await s.loadDays('2026-11-08', '2026-11-09')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/me/nutrition/days?from=2026-11-08&to=2026-11-09')
    expect(Object.keys(s.days)).toEqual(['2026-11-08', '2026-11-09'])
  })

  it('adds a food log entry with POST', async () => {
    const spy = mockFetch(201, { id: 'l1' })
    await useNutrition().addLog({ date: '2026-11-08', productId: 'p', quantity: 150, unit: 'g' })
    expect(spy.mock.calls[0][0]).toBe('/api/v1/me/food-log')
    expect(spy.mock.calls[0][1]!.method).toBe('POST')
    expect(JSON.parse(String(spy.mock.calls[0][1]!.body))).toEqual({ date: '2026-11-08', productId: 'p', quantity: 150, unit: 'g' })
  })

  it('removes a food log entry with DELETE', async () => {
    const spy = mockFetch(204, null)
    await useNutrition().removeLog('l 1')
    expect(spy.mock.calls[0][0]).toBe('/api/v1/me/food-log/l%201')
    expect(spy.mock.calls[0][1]!.method).toBe('DELETE')
  })
})
