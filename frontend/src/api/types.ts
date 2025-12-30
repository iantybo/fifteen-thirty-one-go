export type User = {
  id: number
  username: string
  created_at?: string
}

export type AuthResponse = {
  user: User
}

export type Lobby = {
  id: number
  name: string
  host_id: number
  max_players: number
  current_players: number
  status: 'waiting' | 'in_progress' | 'finished'
  created_at: string
}

export type Game = {
  id: number
  lobby_id: number
  status: 'waiting' | 'in_progress' | 'finished'
  current_player_id?: number
  dealer_id?: number
  created_at: string
  finished_at?: string
}

export type GamePlayer = {
  game_id: number
  user_id: number
  position: number
  score: number
  hand: string
  crib_cards?: string
  is_bot: boolean
  bot_difficulty?: string
}

export type Card = {
  rank: number
  suit: 'S' | 'H' | 'D' | 'C'
}

export type CribbageRules = {
  max_players: number
}

export type CribbageStage = 'dealing' | 'discard' | 'pegging' | 'counting' | 'finished'

export type CribbageState = {
  rules: CribbageRules
  dealer_index: number
  current_index: number
  last_play_index: number
  cut?: Card
  hands: Card[][] // other players are [] (hidden); your hand is populated
  kept_hands?: Card[][] // revealed during counting/finished
  crib?: Card[] // revealed during counting/finished
  pegging_total: number
  pegging_seq: Card[]
  pegging_passed: boolean[]
  discard_completed: boolean[]
  scores: number[]
  stage: CribbageStage
  count_summary?: {
    order: number[]
    hands?: Record<
      string,
      { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    >
    crib?: { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
  }

  history?: Array<{
    round: number
    dealer_index: number
    cut?: Card
    hands?: Record<
      string,
      { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    >
    crib?: { total: number; fifteens: number; pairs: number; runs: number; flush: number; nobs: number; reasons?: Record<string, number> }
    scores_before?: number[]
    scores_after?: number[]
  }>
}

export type GameSnapshot = {
  game: Game
  players: GamePlayer[]
  state: CribbageState
}


