import { describe, expect, it } from 'vitest'
import { resolveRoute } from './guard'

const me = { id: 'u1' }

describe('resolveRoute', () => {
  it('forces setup while no users exist', () => {
    expect(resolveRoute({ me: null, setupRequired: true }, 'login', true)).toEqual({ name: 'setup' })
    expect(resolveRoute({ me: null, setupRequired: true }, 'setup', true)).toBe(true)
  })
  it('hides setup once it is done', () => {
    expect(resolveRoute({ me: null, setupRequired: false }, 'setup', true)).toEqual({ name: 'login' })
    expect(resolveRoute({ me, setupRequired: false }, 'setup', true)).toEqual({ name: 'home' })
  })
  it('sends anonymous users to login but lets public pages through', () => {
    expect(resolveRoute({ me: null, setupRequired: false }, 'home', false)).toEqual({ name: 'login' })
    expect(resolveRoute({ me: null, setupRequired: false }, 'invite', true)).toBe(true)
  })
  it('keeps signed-in users out of the login page', () => {
    expect(resolveRoute({ me, setupRequired: false }, 'login', true)).toEqual({ name: 'home' })
    expect(resolveRoute({ me, setupRequired: false }, 'members', false)).toBe(true)
  })
})
