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
  const headers: Record<string, string> = {
    ...(opts.headers ?? {}),
  }
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`
  if (opts.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json'

  const res = await fetch(url, { ...opts, headers })
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
    return (await res.json()) as T
  }
  throw new ApiError('Unexpected non-JSON response', res.status)
}


