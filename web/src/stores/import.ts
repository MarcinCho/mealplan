import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { RecipeDraft } from '../api/types'

// Holds the draft between the import screen and the recipe editor.
export const useImport = defineStore('import', {
  state: () => ({ draft: null as RecipeDraft | null, targetId: '' }),
  actions: {
    async fromUrl(url: string): Promise<RecipeDraft> {
      this.draft = null
      this.draft = await api<RecipeDraft>('POST', '/recipes/import/url', { url })
      return this.draft
    },
    async fromText(text: string): Promise<RecipeDraft> {
      this.draft = null
      this.draft = await api<RecipeDraft>('POST', '/recipes/import/text', { text })
      return this.draft
    },
    clear() {
      this.draft = null
      this.targetId = ''
    },
  },
})
