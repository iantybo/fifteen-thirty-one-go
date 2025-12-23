export function apiBaseUrl(): string {
  const v = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? ''
  return v.trim().replace(/\/+$/, '')
}

export function wsBaseUrl(): string {
  const v = (import.meta.env.VITE_WS_BASE_URL as string | undefined) ?? ''
  return v.trim().replace(/\/+$/, '')
}


