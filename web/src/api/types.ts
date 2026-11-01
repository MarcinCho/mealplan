export type Slot = 'breakfast' | 'lunch' | 'dinner' | 'snack'

export interface Ingredient {
  name: string
  quantity: number | null
  unit: string
}

export interface Recipe {
  id: string
  title: string
  baseServings: number
  sourceUrl: string
  tags: string[]
  ingredients: Ingredient[]
  steps: string[]
}

export interface PlanEntry {
  id: string
  date: string
  slot: Slot
  recipeId: string | null
  freeText: string
  servings: number
  cookUserId: string | null
}

export interface ShoppingItem {
  id: string
  week: string
  name: string
  quantity: number | null
  unit: string
  checked: boolean
  manual: boolean
}
