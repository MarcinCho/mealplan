import { describe, expect, it } from 'vitest'
import { mountWith } from '../../test/mount'
import CookBadge from './CookBadge.vue'

describe('CookBadge', () => {
  it('shows the initial and the name', () => {
    const w = mountWith(CookBadge, { props: { name: 'Tomek', color: '#123456' } })
    expect(w.attributes('title')).toBe('Tomek')
    const av = w.find('.cook-badge__avatar')
    expect(av.text()).toBe('T')
    expect((av.element as HTMLElement).style.backgroundColor).toBe('rgb(18, 52, 86)')
    expect(w.text()).toContain('Tomek')
  })

  it('can hide the visible name', () => {
    const w = mountWith(CookBadge, { props: { name: 'Tomek', color: '#123456', showName: false } })
    const clone = w.element.cloneNode(true) as HTMLElement
    clone.querySelectorAll('.d-sr-only').forEach((e) => e.remove())
    expect(clone.textContent?.trim()).toBe('T')
    expect(w.attributes('title')).toBe('Tomek')
    expect(w.find('.d-sr-only').text()).toBe('Tomek')
  })
})
