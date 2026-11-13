// isStaleChunk tells whether a navigation failed because a lazily loaded view chunk is gone,
// which happens to a tab opened before a deploy.
export function isStaleChunk(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err)
  return /dynamically imported module|Importing a module script failed|error loading dynamically imported/i.test(msg)
}
