import { describe, expect, it } from 'vitest'
import { mountWith } from '../../test/mount'
import NoteCard from './NoteCard.vue'

describe('NoteCard', () => {
  it('renders the label and title in the note header', () => {
    const w = mountWith(NoteCard, { props: { label: 'Dziś na obiad', title: 'Zupa ogórkowa' } })
    expect(w.find('.note-card__header').classes()).toContain('bg-note')
    expect(w.find('.note-card__label').text()).toBe('Dziś na obiad')
    expect(w.find('h2.note-card__title').text()).toBe('Zupa ogórkowa')
  })

  it('uses the requested heading level', () => {
    const w = mountWith(NoteCard, { props: { title: 'Zupa', titleTag: 'h1' } })
    expect(w.find('h1.note-card__title').exists()).toBe(true)
    expect(w.find('h2').exists()).toBe(false)
  })

  it('fills the slots', () => {
    const w = mountWith(NoteCard, {
      slots: {
        title: '<span class="x">Gulasz</span>',
        actions: '<button>+</button>',
        subheader: 'Przeliczone',
        default: '<p>body</p>',
      },
    })
    expect(w.find('.note-card__title .x').text()).toBe('Gulasz')
    expect(w.find('.note-card__actions button').exists()).toBe(true)
    expect(w.find('.note-card__subheader').text()).toBe('Przeliczone')
    expect(w.find('.note-card__body').classes()).toContain('mp-grid')
    expect(w.find('.note-card__body p').exists()).toBe(true)
  })

  it('omits empty parts', () => {
    const w = mountWith(NoteCard)
    for (const s of ['label', 'title', 'actions', 'subheader', 'body']) {
      expect(w.find(`.note-card__${s}`).exists()).toBe(false)
    }
  })
})
