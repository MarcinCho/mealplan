import { describe, expect, it } from 'vitest'
import { isStaleChunk } from './chunks'

describe('isStaleChunk', () => {
  it('recognises the browsers’ chunk-load errors', () => {
    expect(isStaleChunk(new TypeError('Failed to fetch dynamically imported module: https://x/assets/PlanView-abc.js'))).toBe(true)
    expect(isStaleChunk(new TypeError('Importing a module script failed.'))).toBe(true)
    expect(isStaleChunk(new Error('error loading dynamically imported module'))).toBe(true)
  })
  it('ignores other errors', () => {
    expect(isStaleChunk(new Error('network down'))).toBe(false)
  })
})
