import type { Card } from '../../api/types'

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

export function suitColor(suit: Card['suit']): string {
  return suit === 'H' || suit === 'D' ? '#dc2626' : '#0f172a'
}

export function cardToCode(c: Card): string {
  return `${rankLabel(c.rank)}${c.suit}`
}

export function cardToString(c: Card): string {
  return `${rankLabel(c.rank)}${suitSymbol(c.suit)}`
}

export function cardValue15(c: Card): number {
  // Pegging total uses 15-values: A=1, 2-10 as-is, J/Q/K=10.
  return c.rank >= 10 ? 10 : c.rank
}
