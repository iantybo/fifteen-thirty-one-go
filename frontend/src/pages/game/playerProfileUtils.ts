import type { UserStats } from '../../api/types'

export type ScoreBreakdown = {
  total: number
  fifteens: number
  pairs: number
  runs: number
  flush: number
  nobs: number
  reasons?: Record<string, number>
}

export type PlayerProfileState = {
  loading: boolean
  stats?: UserStats
  error?: string
}

export function playerInitials(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return '?'
  const parts = trimmed.split(/\s+/).slice(0, 2)
  const letters = parts.map((p) => p[0]).join('')
  return letters.toUpperCase()
}

export function profilePalette(slot: number) {
  const palettes = [
    { bg1: '#fef3c7', bg2: '#fde68a', accent: '#b45309', border: '#f59e0b' },
    { bg1: '#dbeafe', bg2: '#bfdbfe', accent: '#1d4ed8', border: '#60a5fa' },
    { bg1: '#dcfce7', bg2: '#bbf7d0', accent: '#15803d', border: '#4ade80' },
    { bg1: '#fee2e2', bg2: '#fecaca', accent: '#b91c1c', border: '#f87171' },
  ]
  return palettes[Math.abs(slot) % palettes.length]
}
