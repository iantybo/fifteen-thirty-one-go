export type ApiErrorBody = { error?: string }

export class ApiError extends Error {
  status: number
  body?: ApiErrorBody
  constructor(message: string, status: number, body?: ApiErrorBody) {
    super(message)
    this.status = status
    this.body = body
  }
}

export type RequestOptions = Omit<RequestInit, 'headers'> & {
  token?: string | null
  headers?: Record<string, string>
}

export async function apiFetch<T>(url: string, opts: RequestOptions = {}): Promise<T> {
  const { token, headers: headersOpt, credentials, ...fetchOpts } = opts
  const headers: Record<string, string> = {
    ...(headersOpt ?? {}),
  }
  if (token) headers.Authorization = `Bearer ${token}`
  if (opts.body) {
    // Case-insensitive header check to avoid duplicates like "content-type" + "Content-Type".
    const existingContentTypeKey = Object.keys(headers).find((k) => k.toLowerCase() === 'content-type')
    if (existingContentTypeKey) {
      headers[existingContentTypeKey] = 'application/json'
    } else {
      headers['Content-Type'] = 'application/json'
    }
  }

  // Default to credentialed requests so server-set httpOnly cookies (sessions) are sent.
  const res = await fetch(url, { ...fetchOpts, headers, credentials: credentials ?? 'include' })
  const contentType = res.headers.get('content-type') ?? ''

  const parseJson = async () => {
    try {
      return (await res.json()) as unknown
    } catch {
      return undefined
    }
  }

  if (!res.ok) {
    const body = contentType.includes('application/json') ? ((await parseJson()) as ApiErrorBody) : undefined
    const msg = body?.error ?? `Request failed (${res.status})`
    throw new ApiError(msg, res.status, body)
  }

  if (contentType.includes('application/json')) {
    try {
      return (await res.json()) as T
    } catch {
      throw new ApiError(`Failed to parse JSON response (${res.status})`, res.status)
    }
  }
  throw new ApiError('Unexpected non-JSON response', res.status)
}


