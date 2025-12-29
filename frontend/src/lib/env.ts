export function apiBaseUrl(): string {
  const raw = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? ''
  const baseUrl = raw.trim().replace(/\/+$/, '')

  if (baseUrl === '') {
    throw new Error(
      'VITE_API_BASE_URL is required and must be non-empty. ' +
        'Set it to your backend origin (e.g. "http://127.0.0.1:8080").',
    )
  }

  let u: URL
  try {
    // Validate it's an absolute URL (will throw on invalid/missing protocol, etc).
    u = new URL(baseUrl)
  } catch {
    throw new Error(
      `VITE_API_BASE_URL is invalid: expected an absolute URL like "http://127.0.0.1:8080". Got ${JSON.stringify(
        baseUrl,
      )}`,
    )
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') {
    throw new Error(
      `VITE_API_BASE_URL must use http:// or https://. Got protocol ${JSON.stringify(u.protocol)} from ${JSON.stringify(
        baseUrl,
      )}`,
    )
  }

  return baseUrl
}

export function wsBaseUrl(): string {
  const raw = (import.meta.env.VITE_WS_BASE_URL as string | undefined) ?? ''
  const baseUrl = raw.trim().replace(/\/+$/, '')

  if (baseUrl === '') {
    throw new Error(
      'VITE_WS_BASE_URL is required and must be non-empty. ' +
        'Set it to your backend WebSocket origin (e.g. "ws://127.0.0.1:8080").',
    )
  }

  let u: URL
  try {
    u = new URL(baseUrl)
  } catch {
    throw new Error(
      `VITE_WS_BASE_URL is invalid: expected an absolute WebSocket URL like "ws://127.0.0.1:8080" or "wss://example.com". Got ${JSON.stringify(
        baseUrl,
      )}`,
    )
  }

  if (u.protocol !== 'ws:' && u.protocol !== 'wss:') {
    throw new Error(
      `VITE_WS_BASE_URL is invalid: expected a ws:// or wss:// URL. Got protocol ${JSON.stringify(
        u.protocol,
      )} from ${JSON.stringify(baseUrl)}`,
    )
  }

  return baseUrl
}


