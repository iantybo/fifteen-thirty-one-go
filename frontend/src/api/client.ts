import { apiBaseUrl } from '../lib/env'
import { apiFetch } from '../lib/http'
import type { AuthResponse, Lobby } from './types'

export type RegisterRequest = { username: string; password: string }
export type LoginRequest = { username: string; password: string }
export type CreateLobbyRequest = { name?: string; max_players: number }

export const api = {
  register(req: RegisterRequest) {
    return apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/register`, {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },
  login(req: LoginRequest) {
    return apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/login`, {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },
  listLobbies(token: string) {
    return apiFetch<{ lobbies: Lobby[] }>(`${apiBaseUrl()}/api/lobbies`, { token })
  },
  createLobby(token: string, req: CreateLobbyRequest) {
    return apiFetch<{ lobby: Lobby; game: { id: number; lobby_id: number; status: string } }>(`${apiBaseUrl()}/api/lobbies`, {
      method: 'POST',
      token,
      body: JSON.stringify(req),
    })
  },
  joinLobby(token: string, lobbyId: number) {
    return apiFetch<{ lobby: Lobby; game_id: number }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/join`, {
      method: 'POST',
      token,
    })
  },
}


