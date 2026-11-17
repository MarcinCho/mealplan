import { describe, expect, it } from 'vitest'
import { parseServings, scaleAmount } from './scale'

describe('scaleAmount', () => {
  const cases: [string, number | null, string, number, number | null, string][] = [
    ['factor 1 does not round', 0.3, 'szklanka', 1, 0.3, 'szklanka'],
    ['factor 1 does not round (g)', 333.33, 'g', 1, 333.33, 'g'],
    ['null stays', null, '', 3, null, ''],
    ['zero stays zero', 0, 'g', 3, 0, 'g'],
    ['g to tens', 125, 'g', 1.5, 190, 'g'],
    ['g x3', 125, 'g', 3, 380, 'g'],
    ['g 110 x3', 110, 'g', 3, 330, 'g'],
    ['g small', 4.5, 'g', 3, 14, 'g'],
    ['tablespoon 1.5', 1, 'łyżka', 1.5, 1.5, 'łyżka'],
    ['teaspoon quarter', 1, 'łyżeczka', 0.25, 0.25, 'łyżeczka'],
    ['unitless 1.5', 3, '', 0.5, 1.5, ''],
    ['pinch halved', 1, 'szczypta', 0.5, 1, 'szczypta'],
    ['kg to g', 0.5, 'kg', 0.5, 250, 'g'],
    ['g to 0.5', 3.3, 'g', 1.5, 5, 'g'],
    ['ml minimum', 0.1, 'ml', 2, 0.5, 'ml'],
    ['g 9.9', 9.9, 'g', 1.01, 10, 'g'],
    ['g band recheck to kg', 998, 'g', 1.0001, 1, 'kg'],
    ['ml to l', 800, 'ml', 2, 1.6, 'l'],
    ['g to kg step', 1234, 'g', 2, 2.45, 'kg'],
    ['kg stays kg', 0.999, 'kg', 1.0001, 1, 'kg'],
    ['l to ml', 0.4, 'l', 2, 800, 'ml'],
    ['kg step 0.05', 1.33, 'kg', 1.5, 2, 'kg'],
    ['spoon 0.25 grid', 1, 'łyżka', 0.4, 0.5, 'łyżka'],
    ['spoon minimum', 1, 'łyżeczka', 0.1, 0.25, 'łyżeczka'],
    ['cup 0.5 grid', 3, 'szklanka', 0.3, 1, 'szklanka'],
    ['piece minimum', 1, 'szt', 0.25, 0.5, 'szt'],
    ['bunch', 0.5, 'pęczek', 2, 1, 'pęczek'],
    ['clove', 2, 'ząbek', 1.3, 2.5, 'ząbek'],
    ['pinch x3', 1, 'szczypta', 3, 3, 'szczypta'],
    ['pinch minimum', 1, 'szczypta', 0.2, 1, 'szczypta'],
    ['unknown unit 0.1', 3, 'cm', 0.33, 1, 'cm'],
    ['unknown unit minimum', 0.1, 'cm', 0.1, 0.1, 'cm'],
    ['unknown unit whole', 7, 'cm', 2, 14, 'cm'],
    ['g 10 stays g', 10, 'g', 1.0001, 10, 'g'],
    ['g 999 rolls to kg', 999, 'g', 1.0001, 1, 'kg'],
    ['g 1000 is kg', 1000, 'g', 1.0001, 1, 'kg'],
    ['kg 0.99 drops to g', 0.99, 'kg', 1.0001, 990, 'g'],
    ['g 99.5 rounds to 100', 99.5, 'g', 1.0001, 100, 'g'],
    ['unit named like an Object.prototype key', 1000, 'constructor', 2, 2000, 'constructor'],
  ]
  it.each(cases)('%s', (_name, q, unit, f, quantity, outUnit) => {
    expect(scaleAmount(q, unit, f)).toEqual({ quantity, unit: outUnit })
  })
})

describe('parseServings', () => {
  it('accepts whole numbers 1-100', () => {
    expect(parseServings('4', 2)).toBe(4)
    expect(parseServings('100', 2)).toBe(100)
    expect(parseServings('1', 2)).toBe(1)
  })

  it.each([undefined, null, '', 'abc', '0', '101', '2.5', '-3', ' 4', ['3', '5']])(
    'falls back to base for %j',
    (raw) => {
      expect(parseServings(raw, 2)).toBe(2)
    },
  )
})
