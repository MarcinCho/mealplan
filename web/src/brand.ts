// Picks the wordmark variant that stays readable on a given background:
// 'dark' is the wordmark with light text (for dark backgrounds), 'light' the one with dark text.
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

export function logoVariantFor(backgroundHex: string): 'light' | 'dark' {
  return luminance(backgroundHex) < 0.4 ? 'dark' : 'light'
}
