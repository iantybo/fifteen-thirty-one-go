import type { Card } from '../api/types'

// Standard poker-size playing cards are 2.5" x 3.5" (ratio 5:7). Keep our UI cards at that ratio.
export const CARD_W = 70
export const CARD_H = 98
export const CARD_R = 12

export function rankLabel(rank: number): string {
  return rank === 1 ? 'A' : rank === 11 ? 'J' : rank === 12 ? 'Q' : rank === 13 ? 'K' : String(rank)
}

export function suitSymbol(suit: Card['suit']): string {
  switch (suit) {
    case 'S':
      return '♠'
    case 'H':
      return '♥'
    case 'D':
      return '♦'
    case 'C':
      return '♣'
  }
}

/** Compact machine-ish code, e.g. "AS" — used for titles and React keys. */
export function cardToCode(c: Card): string {
  return `${rankLabel(c.rank)}${c.suit}`
}

/** Human-readable label with the suit glyph, e.g. "A♠". */
export function cardToString(c: Card): string {
  return `${rankLabel(c.rank)}${suitSymbol(c.suit)}`
}
