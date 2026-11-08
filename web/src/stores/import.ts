import { defineStore } from 'pinia'
import { api } from '../api/client'
import type { RecipeDraft } from '../api/types'

// Holds the draft between the import screen and the recipe editor.
export const useImport = defineStore('import', {
  state: () => ({ draft: null as RecipeDraft | null, targetId: '' }),
  actions: {
    // The caller decides when (and whether) the draft is stored, so a late response cannot overwrite a newer one.
    fetchUrl(url: string): Promise<RecipeDraft> {
      return api<RecipeDraft>('POST', '/recipes/import/url', { url })
    },
    fetchText(text: string): Promise<RecipeDraft> {
      return api<RecipeDraft>('POST', '/recipes/import/text', { text })
    },
    setDraft(d: RecipeDraft) {
      this.draft = d
    },
    clear() {
      this.draft = null
      this.targetId = ''
    },
  },
})
