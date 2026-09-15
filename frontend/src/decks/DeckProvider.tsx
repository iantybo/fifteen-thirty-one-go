import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api, type DeckRequest } from '../api/client'
import type { DecksResponse } from '../api/types'
import { useAuth } from '../auth/auth'
import { DEFAULT_DECK_THEME, allDeckThemes, resolveDeckTheme, type DeckTheme } from './deckTheme'

type DeckState = {
  /** The deck skin the current user has selected. */
  theme: DeckTheme
  /** Built-in and custom decks available to select. */
  available: DeckTheme[]
  decks: DecksResponse | undefined
  loading: boolean
  error?: string
  refresh: () => Promise<void>
  selectDeck: (ref: string) => Promise<void>
  createDeck: (req: DeckRequest) => Promise<void>
  updateDeck: (deckId: number, req: DeckRequest) => Promise<void>
  deleteDeck: (deckId: number) => Promise<void>
}

const DeckContext = createContext<DeckState | undefined>(undefined)

function errorMessage(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback
}

export function DeckProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth()
  const [decks, setDecks] = useState<DecksResponse | undefined>(undefined)
  const [activeRef, setActiveRef] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | undefined>(undefined)

  // Decks are per-user, so (re)load whenever the signed-in user changes and
  // drop any previously loaded decks on sign-out.
  const userId = user?.id
  useEffect(() => {
    if (!userId) {
      setDecks(undefined)
      setActiveRef('')
      setError(undefined)
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)
    async function load() {
      try {
        const res = await api.listDecks()
        if (cancelled) return
        setDecks(res)
        setActiveRef(res.active_deck)
        setError(undefined)
      } catch (e) {
        // A deck-load failure must not block gameplay; fall back to the
        // default deck and surface the message in the deck manager.
        if (!cancelled) setError(errorMessage(e, 'Failed to load decks'))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [userId])

  const refresh = useCallback(async () => {
    if (!userId) return
    const res = await api.listDecks()
    setDecks(res)
    setActiveRef(res.active_deck)
  }, [userId])

  const selectDeck = useCallback(async (ref: string) => {
    const prefs = await api.setActiveDeck(ref)
    setActiveRef(prefs.active_deck)
  }, [])

  const createDeck = useCallback(
    async (req: DeckRequest) => {
      await api.createDeck(req)
      await refresh()
    },
    [refresh],
  )

  const updateDeck = useCallback(
    async (deckId: number, req: DeckRequest) => {
      await api.updateDeck(deckId, req)
      await refresh()
    },
    [refresh],
  )

  const deleteDeck = useCallback(
    async (deckId: number) => {
      await api.deleteDeck(deckId)
      // The server resets the selection when the active deck is deleted, so
      // refresh picks up both the list and the new active_deck.
      await refresh()
    },
    [refresh],
  )

  const value = useMemo<DeckState>(
    () => ({
      theme: resolveDeckTheme(decks, activeRef),
      available: decks ? allDeckThemes(decks) : [DEFAULT_DECK_THEME],
      decks,
      loading,
      error,
      refresh,
      selectDeck,
      createDeck,
      updateDeck,
      deleteDeck,
    }),
    [decks, activeRef, loading, error, refresh, selectDeck, createDeck, updateDeck, deleteDeck],
  )

  return <DeckContext.Provider value={value}>{children}</DeckContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useDeck(): DeckState {
  const ctx = useContext(DeckContext)
  if (!ctx) throw new Error('useDeck must be used within DeckProvider')
  return ctx
}
