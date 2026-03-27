import { useCallback, useMemo, useState } from 'react'

const PEG_BOARD_STORAGE_KEY = 'fto-peg-board-theme'

export type PegBoardThemeId = 'classic' | 'wood' | 'ocean' | 'minimal'

export type PegBoardTheme = {
  id: PegBoardThemeId
  name: string
  trackBg: string
  trackBorder: string
  startCapBg: string
  endCapBg: string
  endCapTextColor: string
  endCapTextShadow: string
  holeColor: string
  holeShadow: string
  tickMajor: string
  tickMinor: string
  labelMajor: string
  labelMinor: string
  pegColors: string[]
  pegBorder: string
  pegShadow: string
  scoreBadgeBg: string
  scoreBadgeBorder: string
  scoreBadgeColor: string
  containerBorder: string
  containerBg: string
}

export const PEG_BOARD_THEMES: PegBoardTheme[] = [
  {
    id: 'classic',
    name: 'Classic',
    trackBg: '#f1f5f9',
    trackBorder: '#e2e8f0',
    startCapBg: '#eef2ff',
    endCapBg: '#0f766e',
    endCapTextColor: '#facc15',
    endCapTextShadow: '0 1px 0 rgba(0,0,0,0.25)',
    holeColor: '15, 23, 42',
    holeShadow: 'inset 0 1px 1px rgba(255,255,255,0.45)',
    tickMajor: '#cbd5e1',
    tickMinor: '#e2e8f0',
    labelMajor: '#0f172a',
    labelMinor: '#334155',
    pegColors: ['#2563eb', '#dc2626', '#16a34a', '#7c3aed'],
    pegBorder: '#ffffff',
    pegShadow: '0 1px 3px rgba(0,0,0,0.25)',
    scoreBadgeBg: 'rgba(255,255,255,0.9)',
    scoreBadgeBorder: '#e2e8f0',
    scoreBadgeColor: '#0f172a',
    containerBorder: '#e2e8f0',
    containerBg: 'transparent',
  },
  {
    id: 'wood',
    name: 'Wood',
    trackBg: 'linear-gradient(180deg, #d4a574 0%, #c4956a 50%, #8b6914 100%)',
    trackBorder: '#6b5344',
    startCapBg: '#e8d5c4',
    endCapBg: '#5d4e37',
    endCapTextColor: '#fef3c7',
    endCapTextShadow: '0 1px 2px rgba(0,0,0,0.5)',
    holeColor: '0, 0, 0',
    holeShadow: 'inset 0 1px 2px rgba(0,0,0,0.4)',
    tickMajor: '#6b5344',
    tickMinor: 'rgba(107,83,68,0.6)',
    labelMajor: '#3d2c1e',
    labelMinor: '#5d4e37',
    pegColors: ['#1e40af', '#b91c1c', '#166534', '#5b21b6'],
    pegBorder: '#fef3c7',
    pegShadow: '0 2px 4px rgba(0,0,0,0.35)',
    scoreBadgeBg: '#fef3c7',
    scoreBadgeBorder: '#b45309',
    scoreBadgeColor: '#451a03',
    containerBorder: '#6b5344',
    containerBg: 'rgba(254,243,199,0.2)',
  },
  {
    id: 'ocean',
    name: 'Ocean',
    trackBg: 'linear-gradient(180deg, #e0f2fe 0%, #bae6fd 50%, #0ea5e9 100%)',
    trackBorder: '#0284c7',
    startCapBg: '#f0f9ff',
    endCapBg: '#0369a1',
    endCapTextColor: '#fef08a',
    endCapTextShadow: '0 1px 0 rgba(0,0,0,0.3)',
    holeColor: '2, 132, 199',
    holeShadow: 'inset 0 1px 1px rgba(255,255,255,0.6)',
    tickMajor: '#0ea5e9',
    tickMinor: 'rgba(14,165,233,0.5)',
    labelMajor: '#0c4a6e',
    labelMinor: '#0369a1',
    pegColors: ['#1d4ed8', '#dc2626', '#059669', '#7c3aed'],
    pegBorder: '#ffffff',
    pegShadow: '0 1px 3px rgba(0,0,0,0.2)',
    scoreBadgeBg: 'rgba(255,255,255,0.95)',
    scoreBadgeBorder: '#0284c7',
    scoreBadgeColor: '#0c4a6e',
    containerBorder: '#0284c7',
    containerBg: 'rgba(224,242,254,0.4)',
  },
  {
    id: 'minimal',
    name: 'Minimal',
    trackBg: '#fafafa',
    trackBorder: '#d4d4d4',
    startCapBg: '#f5f5f5',
    endCapBg: '#262626',
    endCapTextColor: '#fafafa',
    endCapTextShadow: 'none',
    holeColor: '0, 0, 0',
    holeShadow: 'none',
    tickMajor: '#a3a3a3',
    tickMinor: '#d4d4d4',
    labelMajor: '#171717',
    labelMinor: '#525252',
    pegColors: ['#3b82f6', '#ef4444', '#22c55e', '#a855f7'],
    pegBorder: '#fafafa',
    pegShadow: '0 1px 2px rgba(0,0,0,0.1)',
    scoreBadgeBg: '#ffffff',
    scoreBadgeBorder: '#d4d4d4',
    scoreBadgeColor: '#171717',
    containerBorder: '#d4d4d4',
    containerBg: '#fafafa',
  },
]

function getStoredPegBoardTheme(): PegBoardThemeId {
  if (typeof window === 'undefined') return 'classic'
  const raw = window.localStorage.getItem(PEG_BOARD_STORAGE_KEY)
  if (raw === 'classic' || raw === 'wood' || raw === 'ocean' || raw === 'minimal') return raw
  return 'classic'
}

export function usePegBoardTheme(): [PegBoardTheme, (id: PegBoardThemeId) => void] {
  const [themeId, setThemeId] = useState<PegBoardThemeId>(getStoredPegBoardTheme)
  const theme = useMemo(() => PEG_BOARD_THEMES.find((t) => t.id === themeId) ?? PEG_BOARD_THEMES[0], [themeId])
  const setTheme = useCallback((id: PegBoardThemeId) => {
    setThemeId(id)
    try {
      window.localStorage.setItem(PEG_BOARD_STORAGE_KEY, id)
    } catch (e) {
      console.warn('[pegBoardThemes] Failed to persist theme preference', id, e)
    }
  }, [])
  return [theme, setTheme]
}
