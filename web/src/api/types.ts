export type Slot = 'breakfast' | 'lunch' | 'dinner' | 'snack'

export type Confidence = 'ok' | 'guess' | 'unparsed'

export interface Ingredient {
  name: string
  quantity: number | null
  unit: string
  // Saved with the recipe (max 200 characters); an absent value is sent as empty.
  note?: string
  // Import-only hints, used for highlighting and never sent to the API.
  raw?: string
  confidence?: Confidence
}

export interface DraftIngredient extends Ingredient {
  raw: string
  note: string
  confidence: Confidence
}

export interface RecipeDraft {
  title: string
  servings: number
  sourceUrl: string
  tags: string[]
  ingredients: DraftIngredient[]
  steps: string[]
  warnings: string[]
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
  done: boolean
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
