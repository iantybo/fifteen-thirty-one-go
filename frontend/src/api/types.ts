export type User = {
  id: number
  username: string
  created_at?: string
}

export type AuthResponse = {
  token: string
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
  current_player_id?: number | null
  dealer_id?: number | null
  created_at: string
  finished_at?: string | null
}


