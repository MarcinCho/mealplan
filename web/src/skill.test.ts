import { describe, expect, it } from 'vitest'
import text from '../public/claude-skill/SKILL.md?raw'

describe('claude skill download', () => {
  it('is a skill file with frontmatter and the recipe format sections', () => {
    expect(text.startsWith('---')).toBe(true)
    expect(text.split('---')[1]).toContain('name: mealplan-recipe-format')
    expect(text).toContain('Składniki:')
    expect(text).toContain('Przygotowanie:')
  })
})
