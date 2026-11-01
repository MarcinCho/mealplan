import { defineStore } from 'pinia'
import { api, ApiError, setCsrfToken } from '../api/client'

export interface Me {
  user: { id: string; login: string; displayName: string }
  household: { id: string; name: string }
  role: 'admin' | 'member'
  csrfToken: string
}

interface Status {
  setupRequired: boolean
  mode: 'selfhost' | 'hosted'
}

export const useSession = defineStore('session', {
  state: () => ({ me: null as Me | null, loaded: false, setupRequired: false }),
  getters: {
    isAdmin: (s) => s.me?.role === 'admin',
  },
  actions: {
    async bootstrap() {
      const status = await api<Status>('GET', '/auth/status')
      this.setupRequired = status.setupRequired
      try {
        await this.fetchMe()
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 401)) throw e
        this.me = null
      }
      this.loaded = true
    },
    async fetchMe() {
      const me = await api<Me>('GET', '/auth/me')
      this.me = me
      setCsrfToken(me.csrfToken)
    },
    // After a successful sign-in the profile must load; a 401 here means the browser dropped
    // the cookie, typically a Secure cookie served over plain HTTP.
    async afterSignIn() {
      try {
        await this.fetchMe()
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) {
          throw new ApiError(0, 'cookie_not_stored', 'Session cookie was not stored by the browser')
        }
        throw e
      }
    },
    async login(login: string, password: string) {
      await api('POST', '/auth/login', { login, password })
      await this.afterSignIn()
    },
    async setup(input: { householdName: string; login: string; password: string; displayName: string }) {
      await api('POST', '/auth/setup', input)
      await this.afterSignIn()
      this.setupRequired = false
    },
    async acceptInvite(token: string, input: { login: string; password: string; displayName: string }) {
      await api('POST', `/auth/invitations/${encodeURIComponent(token)}/accept`, input)
      await this.afterSignIn()
    },
    async logout() {
      await api('POST', '/auth/logout')
      this.me = null
      setCsrfToken('')
    },
  },
})
