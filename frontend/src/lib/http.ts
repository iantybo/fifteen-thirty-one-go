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

export type RequestOptions = Omit<RequestInit, 'headers' | 'body'> & {
  token?: string | null
  headers?: Record<string, string>
  // Allow callers to pass plain objects/arrays; apiFetch will JSON.stringify them.
  body?: unknown
}

function isArrayBufferViewBody(v: unknown): v is ArrayBufferView {
  return v != null && typeof ArrayBuffer !== 'undefined' && ArrayBuffer.isView(v)
}

export async function apiFetch<T>(url: string, opts: RequestOptions = {}): Promise<T | undefined> {
  const { token, headers: headersOpt, credentials, body, ...fetchOpts } = opts
  const headers: Record<string, string> = {
    ...(headersOpt ?? {}),
  }
  if (token) headers.Authorization = `Bearer ${token}`

  // Case-insensitive header check to avoid duplicates like "content-type" + "Content-Type".
  const existingContentTypeKey = Object.keys(headers).find((k) => k.toLowerCase() === 'content-type')
  const existingContentType = existingContentTypeKey ? headers[existingContentTypeKey] : undefined

  // Normalize body:
  // - If a caller passes a plain object/array, auto-JSON.stringify it (otherwise fetch would send "[object Object]").
  // - Don't touch FormData/Blob/etc (those should not get application/json).
  let finalBody: BodyInit | undefined
  const isObjectBody = typeof body === 'object' && body !== null
  const isJsonLike = isObjectBody && (Array.isArray(body) || Object.prototype.toString.call(body) === '[object Object]')
  const isFormData = typeof FormData !== 'undefined' && body instanceof FormData
  const isUrlSearchParams = typeof URLSearchParams !== 'undefined' && body instanceof URLSearchParams
  const isBlob = typeof Blob !== 'undefined' && body instanceof Blob
  const isArrayBuffer = typeof ArrayBuffer !== 'undefined' && body instanceof ArrayBuffer
  const isArrayBufferView = isArrayBufferViewBody(body)
  const isReadableStream =
    typeof ReadableStream !== 'undefined' && typeof body === 'object' && body !== null && body instanceof ReadableStream

  if (isJsonLike) {
    if (existingContentType && !existingContentType.toLowerCase().includes('application/json')) {
      throw new Error(
        `Request body is an object/array but Content-Type is ${JSON.stringify(
          existingContentType,
        )}. Pass body as JSON.stringify(...) or set Content-Type to application/json.`,
      )
    }
    finalBody = JSON.stringify(body)
    headers[existingContentTypeKey ?? 'Content-Type'] = 'application/json'
  } else if (body != null) {
    if (typeof body === 'string') {
      finalBody = body
    } else if (isFormData || isUrlSearchParams || isBlob || isArrayBuffer || isArrayBufferView || isReadableStream) {
      finalBody = body as BodyInit
    } else {
      throw new Error(
        `Unsupported request body type ${JSON.stringify(
          Object.prototype.toString.call(body),
        )}. Pass a string, FormData, Blob, ArrayBuffer, ArrayBufferView, ReadableStream, or a plain object/array.`,
      )
    }

    // If caller provided a raw body, only default Content-Type when it's plausibly JSON text.
    if (!existingContentTypeKey && typeof body === 'string') {
      const t = body.trim()
      if (t.startsWith('{') || t.startsWith('[')) {
        headers['Content-Type'] = 'application/json'
      }
    }
    // Never default Content-Type for these; browser sets correct headers.
    if (!existingContentTypeKey && (isFormData || isUrlSearchParams || isBlob || isArrayBuffer || isArrayBufferView)) {
      // leave Content-Type unset
    }
  }

  // Default to credentialed requests so server-set httpOnly cookies (sessions) are sent.
  const res = await fetch(url, { ...fetchOpts, headers, body: finalBody, credentials: credentials ?? 'include' })
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

  // Handle empty responses (e.g., 204 No Content, empty 200s).
  // Note: Content-Length may be absent for chunked encoding, so we also treat an empty body as success.
  const contentLength = (res.headers.get('content-length') ?? '').trim()
  if (res.status === 204 || res.status === 205 || contentLength === '0') {
    return undefined
  }

  if (contentType.includes('application/json')) {
    const text = await res.text()
    if (text.trim() === '') return undefined
    try {
      return JSON.parse(text) as T
    } catch {
      throw new ApiError(`Failed to parse JSON response (${res.status})`, res.status)
    }
  }

  // Non-JSON success: only error if the response actually has a body.
  const text = await res.text()
  if (text.trim() === '') return undefined
  throw new ApiError('Unexpected non-JSON response', res.status)
}


