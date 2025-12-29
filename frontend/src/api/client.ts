import { apiBaseUrl } from '../lib/env'
import { apiFetch } from '../lib/http'
import type { AuthResponse, Game, Lobby, User } from './types'

type AuthCredentials = { username: string; password: string }
export type RegisterRequest = AuthCredentials
export type LoginRequest = AuthCredentials
export type CreateLobbyRequest = { name: string; max_players: number }

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
  me() {
    return apiFetch<{ user: User }>(`${apiBaseUrl()}/api/auth/me`)
  },
  logout() {
    // Logout is best-effort; callers shouldn't need to catch.
    return apiFetch<void>(`${apiBaseUrl()}/api/auth/logout`, { method: 'POST' }).catch(() => undefined)
  },
  listLobbies() {
    return apiFetch<{ lobbies: Lobby[] }>(`${apiBaseUrl()}/api/lobbies`)
  },
  createLobby(req: CreateLobbyRequest) {
    return apiFetch<{ lobby: Lobby; game: Game }>(`${apiBaseUrl()}/api/lobbies`, {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },
  joinLobby(lobbyId: number) {
    return apiFetch<{
      lobby: Lobby
      game_id: number
      joined_persisted?: boolean
      realtime_sync?: string
    }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/join`, {
      method: 'POST',
    })
  },
}


