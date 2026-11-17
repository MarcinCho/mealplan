import { describe, expect, it, vi } from 'vitest'
import { mountWith } from '../../test/mount'
import NoteRow from './NoteRow.vue'

describe('NoteRow', () => {
  it('renders the label, content and action', () => {
    const w = mountWith(NoteRow, { props: { label: 'Obiad' }, slots: { default: 'Zupa', action: '<button>v</button>' } })
    expect(w.find('.note-row__label').text()).toBe('Obiad')
    expect(w.find('.note-row__content').text()).toBe('Zupa')
    expect(w.find('.note-row__action button').exists()).toBe(true)
  })

  it('lets the label slot replace the label prop', () => {
    const w = mountWith(NoteRow, { props: { label: 'Obiad' }, slots: { label: '<b>Kolacja</b>' } })
    expect(w.find('.note-row__label').text()).toBe('Kolacja')
  })

  it('shows the highlighted state', () => {
    expect(mountWith(NoteRow, { props: { highlighted: true } }).classes()).toContain('note-row--highlighted')
    expect(mountWith(NoteRow, { props: { highlighted: false } }).classes()).not.toContain('note-row--highlighted')
  })

  it('renders as the given tag', () => {
    expect(mountWith(NoteRow, { props: { tag: 'li' } }).element.tagName).toBe('LI')
  })

  it('omits the action column without an action', () => {
    expect(mountWith(NoteRow, { slots: { default: 'Zupa' } }).find('.note-row__action').exists()).toBe(false)
  })

  it('passes clicks to the root', async () => {
    const spy = vi.fn()
    const w = mountWith(NoteRow, { attrs: { onClick: spy }, slots: { default: 'Zupa' } })
    await w.find('.note-row__content').trigger('click')
    expect(spy).toHaveBeenCalledTimes(1)
  })
})
