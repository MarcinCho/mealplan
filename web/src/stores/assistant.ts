import { defineStore } from 'pinia'
import { api, ApiError } from '../api/client'
import type {
  AssignResult,
  AssistantSettings,
  DuplicateGroup,
  IngredientCandidate,
  IngredientQuery,
  MergeResult,
  ProductUsage,
  Suggestion,
} from '../api/types'

const BATCH = 25
const DEFAULT_WAIT_S = 10
const MAX_WAIT_S = 60
// A rate-limited batch is retried after Retry-After; give up after this many tries so a stuck limiter cannot loop forever.
const MAX_ATTEMPTS = 6

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

// Model endpoints share a per-user rate limit: on 429 wait Retry-After (default 10 s, at most 60 s) and retry the same call.
// onWait receives the seconds left, once a second, while waiting (0 when the wait is over).
export async function withRateLimitRetry<T>(call: () => Promise<T>, onWait?: (secondsLeft: number) => void): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return await call()
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 429 || attempt >= MAX_ATTEMPTS) throw e
      const wait = Math.min(e.retryAfter || DEFAULT_WAIT_S, MAX_WAIT_S)
      for (let left = wait; left > 0; left--) {
        onWait?.(left)
        await sleep(1000)
      }
      onWait?.(0)
    }
  }
}

export const useAssistant = defineStore('assistant', {
  state: () => ({
    configured: false,
    keyHint: '',
    model: '',
    defaultModel: '',
    loaded: false,
    // Seconds left of a 429 Retry-After wait, 0 when not waiting.
    waitingSeconds: 0,
  }),
  actions: {
    apply(s: AssistantSettings) {
      this.configured = s.configured
      this.keyHint = s.keyHint
      this.model = s.model
      this.defaultModel = s.defaultModel
      this.loaded = true
    },
    async load() {
      this.apply(await api<AssistantSettings>('GET', '/household/assistant'))
    },
    // key undefined keeps the current key, '' removes it.
    async save(key: string | undefined, model: string) {
      const body: { key?: string; model: string } = { model }
      if (key !== undefined) body.key = key
      this.apply(await api<AssistantSettings>('PUT', '/household/assistant', body))
    },
    async remove() {
      await api('DELETE', '/household/assistant')
      await this.load()
    },
    retry<T>(call: () => Promise<T>): Promise<T> {
      return withRateLimitRetry(call, (n) => {
        this.waitingSeconds = n
      })
    },
    async test() {
      await this.retry(() => api('POST', '/household/assistant/test'))
    },
    async matchIngredients(qs: IngredientQuery[]): Promise<Suggestion[]> {
      const out: Suggestion[] = []
      for (let i = 0; i < qs.length; i += BATCH) {
        const batch = qs.slice(i, i + BATCH)
        const res = await this.retry(() =>
          api<{ suggestions: Suggestion[] }>('POST', '/assistant/ingredients', { ingredients: batch }),
        )
        out.push(...res.suggestions)
      }
      return out
    },
    async candidates(): Promise<IngredientCandidate[]> {
      return (await api<{ candidates: IngredientCandidate[] }>('GET', '/assistant/ingredient-candidates')).candidates
    },
    async productBatches(): Promise<string[][]> {
      return (await api<{ batches: string[][] }>('GET', '/assistant/product-batches')).batches
    },
    async duplicates(ids: string[]): Promise<DuplicateGroup[]> {
      const res = await this.retry(() =>
        api<{ groups: DuplicateGroup[] }>('POST', '/assistant/duplicates', { productIds: ids }),
      )
      return res.groups
    },
    assign(productId: string, name: string, recipeIds: string[]): Promise<AssignResult> {
      return api<AssignResult>('POST', `/products/${encodeURIComponent(productId)}/assign`, { name, recipeIds })
    },
    usage(id: string): Promise<ProductUsage> {
      return api<ProductUsage>('GET', `/products/${encodeURIComponent(id)}/usage`)
    },
    merge(keepId: string, absorbIds: string[]): Promise<MergeResult> {
      return api<MergeResult>('POST', `/products/${encodeURIComponent(keepId)}/merge`, { absorbIds })
    },
  },
})
