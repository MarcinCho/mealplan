import { defineStore } from 'pinia'
import { api } from '../api/client'

export interface Member {
  id: string
  displayName: string
}

export const useMembers = defineStore('members', {
  state: () => ({ items: [] as Member[], loaded: false }),
  getters: {
    ids: (s) => s.items.map((m) => m.id),
    name: (s) => (id: string | null) => s.items.find((m) => m.id === id)?.displayName ?? '',
  },
  actions: {
    async load() {
      this.items = await api<Member[]>('GET', '/household/members')
      this.loaded = true
    },
  },
})
