export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public retryAfter?: number,
  ) {
    super(message)
  }
}

let csrfToken = ''

export function setCsrfToken(token: string) {
  csrfToken = token
}

export async function api<T = void>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken
  const res = await fetch('/api/v1' + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(
      res.status,
      data.code ?? 'unknown',
      data.message ?? res.statusText,
      Number(res.headers.get('Retry-After')) || undefined,
    )
  }
  return data as T
}
