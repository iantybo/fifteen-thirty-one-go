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
  crib_cards?: string | null
  is_bot: boolean
  bot_difficulty?: string | null
}

export type GameSnapshot = {
  game: Game
  players: GamePlayer[]
  // Backend sends a cribbage state object; keep as unknown until we model it.
  state: unknown
}


