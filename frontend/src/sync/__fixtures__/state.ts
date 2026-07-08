/**
 * Test fixtures for the sync engine. A minimal two-player cribbage state plus
 * helpers to build cards and snapshots, so each test file constructs realistic
 * inputs without repeating boilerplate.
 */

import type { Card, CribbageState, GamePlayer, Game, GameSnapshot } from '../../api/types'

/** Build a card from a compact code like "5H", "10S", "KD", "AC". */
export function card(code: string): Card {
  const suit = code.slice(-1) as Card['suit']
  const rankStr = code.slice(0, -1)
  const rank =
    rankStr === 'A' ? 1 : rankStr === 'J' ? 11 : rankStr === 'Q' ? 12 : rankStr === 'K' ? 13 : Number(rankStr)
  return { rank, suit }
}

/** Build a hand from a list of card codes. */
export function hand(...codes: string[]): Card[] {
  return codes.map(card)
}

/**
 * A two-player state in the pegging stage where it is player 0's turn, with a
 * running total the caller can override. Player 0 holds 5H, 6H; player 1's hand
 * is hidden (empty), as the server would send it.
 */
export function peggingState(overrides: Partial<CribbageState> = {}): CribbageState {
  const base: CribbageState = {
    rules: { max_players: 2 },
    dealer_index: 1,
    current_index: 0,
    last_play_index: 1,
    cut: card('QD'),
    hands: [hand('5H', '6H'), []],
    pegging_total: 10,
    pegging_seq: [card('KC')],
    pegging_passed: [false, false],
    discard_completed: [true, true],
    scores: [12, 20],
    stage: 'pegging',
  }
  return { ...base, ...overrides }
}

/** A two-player state in the discard stage; player 0 holds 6 cards. */
export function discardState(overrides: Partial<CribbageState> = {}): CribbageState {
  const base: CribbageState = {
    rules: { max_players: 2 },
    dealer_index: 0,
    current_index: 0,
    last_play_index: -1,
    hands: [hand('AH', '2H', '3H', '4H', '5H', '6H'), []],
    pegging_total: 0,
    pegging_seq: [],
    pegging_passed: [false, false],
    discard_completed: [false, false],
    scores: [0, 0],
    stage: 'discard',
  }
  return { ...base, ...overrides }
}

/** A counting-stage state with a readiness vector. */
export function countingState(overrides: Partial<CribbageState> = {}): CribbageState {
  const base: CribbageState = {
    rules: { max_players: 2 },
    dealer_index: 0,
    current_index: 0,
    last_play_index: 1,
    cut: card('QD'),
    hands: [[], []],
    kept_hands: [hand('5H', '5S', '5D', '5C'), hand('AH', '2H', '3H', '4H')],
    crib: hand('7H', '8H', '9H', '10H'),
    pegging_total: 0,
    pegging_seq: [],
    pegging_passed: [false, false],
    discard_completed: [true, true],
    ready_next_hand: [false, false],
    scores: [80, 60],
    stage: 'counting',
  }
  return { ...base, ...overrides }
}

/** Wrap a state in a full snapshot for two players (ids 9 and 10). */
export function snapshotFor(state: CribbageState, myUserId = 9): GameSnapshot {
  const game: Game = {
    id: 1,
    lobby_id: 1,
    status: 'in_progress',
    created_at: '2026-01-01T00:00:00Z',
  }
  const players: GamePlayer[] = [
    { game_id: 1, user_id: myUserId, username: 'me', position: 0, score: state.scores[0] ?? 0, hand: '[]', is_bot: false },
    { game_id: 1, user_id: myUserId + 1, username: 'them', position: 1, score: state.scores[1] ?? 0, hand: '[]', is_bot: false },
  ]
  return { game, players, state }
}
