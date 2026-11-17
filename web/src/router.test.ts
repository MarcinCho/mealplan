import { describe, expect, it } from 'vitest'
import { router } from './router'

describe('router', () => {
  it.each([
    ['/recipes/r1', 'recipe'],
    ['/recipes/r1/edit', 'recipe-edit'],
    ['/recipes/new', 'recipe-new'],
    ['/recipes/check', 'recipe-check'],
    ['/recipes/import', 'recipe-import'],
  ])('resolves %s to %s', (path, name) => {
    expect(router.resolve(path).name).toBe(name)
  })
})
