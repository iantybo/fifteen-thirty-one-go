import type { BuiltinDeck, Card, CardDeck, DecksResponse } from '../api/types'
import { rankLabel } from './cardLabels'

/** Resolved deck appearance used by the card components. */
export type DeckTheme = {
  /** "builtin:<id>", a custom deck id as a string, or "" for the default deck. */
  ref: string
  name: string
  backImageUrl?: string
  faceImageTemplate?: string
  redSuitColor: string
  blackSuitColor: string
  borderColor: string
}

export const DEFAULT_DECK_THEME: DeckTheme = {
  ref: '',
  name: 'Classic',
  redSuitColor: '#dc2626',
  blackSuitColor: '#0f172a',
  borderColor: '#cbd5e1',
}

export const BUILTIN_DECK_PREFIX = 'builtin:'
export const DEFAULT_BUILTIN_DECK_ID = 'classic'

/**
 * Only absolute https URLs are rendered. The backend enforces this too, but
 * re-checking here means a deck row written before validation (or by a future
 * import path) still cannot inject a javascript:/data: URL into the DOM.
 */
export function safeImageUrl(raw: string | undefined): string | undefined {
  if (!raw) return undefined
  try {
    const u = new URL(raw)
    return u.protocol === 'https:' ? u.toString() : undefined
  } catch {
    return undefined
  }
}

/**
 * Expands a face-image template for one card. Supported placeholders:
 * `{rank}` (A,2..10,J,Q,K), `{suit}` (S/H/D/C) and `{code}` (e.g. "AS").
 * Values are URL-encoded, and the result must still be an https URL.
 */
export function faceImageUrlFor(theme: DeckTheme, card: Card): string | undefined {
  const template = theme.faceImageTemplate
  if (!template) return undefined

  const rank = rankLabel(card.rank)
  const expanded = template
    .replaceAll('{rank}', encodeURIComponent(rank))
    .replaceAll('{suit}', encodeURIComponent(card.suit))
    .replaceAll('{code}', encodeURIComponent(`${rank}${card.suit}`))

  return safeImageUrl(expanded)
}

/** Returns the color to draw a suit's pips and rank labels in. */
export function suitColorFor(theme: DeckTheme, suit: Card['suit']): string {
  return suit === 'H' || suit === 'D' ? theme.redSuitColor : theme.blackSuitColor
}

function builtinToTheme(d: BuiltinDeck): DeckTheme {
  return {
    ref: d.id === DEFAULT_BUILTIN_DECK_ID ? '' : `${BUILTIN_DECK_PREFIX}${d.id}`,
    name: d.name,
    backImageUrl: safeImageUrl(d.back_image_url),
    faceImageTemplate: d.face_image_template,
    redSuitColor: d.red_suit_color,
    blackSuitColor: d.black_suit_color,
    borderColor: d.border_color,
  }
}

function customToTheme(d: CardDeck): DeckTheme {
  return {
    ref: String(d.id),
    name: d.name,
    backImageUrl: safeImageUrl(d.back_image_url),
    faceImageTemplate: d.face_image_template,
    redSuitColor: d.red_suit_color,
    blackSuitColor: d.black_suit_color,
    borderColor: d.border_color,
  }
}

/** Every selectable deck, built-ins first, as resolved themes. */
export function allDeckThemes(decks: DecksResponse | undefined): DeckTheme[] {
  if (!decks) return [DEFAULT_DECK_THEME]
  return [...decks.builtin.map(builtinToTheme), ...decks.custom.map(customToTheme)]
}

/**
 * Resolves the active deck reference against the available decks, falling back
 * to the default theme when the reference is unknown (e.g. a deck deleted in
 * another tab).
 */
export function resolveDeckTheme(decks: DecksResponse | undefined, ref: string): DeckTheme {
  const normalized = ref === `${BUILTIN_DECK_PREFIX}${DEFAULT_BUILTIN_DECK_ID}` ? '' : ref
  if (!normalized) return DEFAULT_DECK_THEME
  return allDeckThemes(decks).find((t) => t.ref === normalized) ?? DEFAULT_DECK_THEME
}
