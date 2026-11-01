export interface GuardState {
  me: unknown | null
  setupRequired: boolean
}

export function resolveRoute(state: GuardState, name: string | undefined, isPublic: boolean): true | { name: string } {
  if (state.setupRequired) return name === 'setup' ? true : { name: 'setup' }
  if (name === 'setup') return state.me ? { name: 'home' } : { name: 'login' }
  if (!state.me) return isPublic ? true : { name: 'login' }
  if (name === 'login') return { name: 'home' }
  return true
}
