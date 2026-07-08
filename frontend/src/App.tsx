import { Navigate, Route, Routes } from 'react-router-dom'
import './App.css'
import { RequireAuth } from './routes/RequireAuth'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'
import { LobbiesPage } from './pages/LobbiesPage'
import { CreateLobbyPage } from './pages/CreateLobbyPage'
import { LobbyDetailPage } from './pages/LobbyDetailPage'
import { GamePage } from './pages/GamePage'
import { LeaderboardPage } from './pages/LeaderboardPage'
import { PostersPage } from './pages/PostersPage'

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/posters" element={<PostersPage />} />

      <Route element={<RequireAuth />}>
        <Route path="/" element={<Navigate to="/lobbies" replace />} />
        <Route path="/lobbies" element={<LobbiesPage />} />
        <Route path="/lobbies/new" element={<CreateLobbyPage />} />
        <Route path="/leaderboard" element={<LeaderboardPage />} />
        <Route path="/lobbies/:id" element={<LobbyDetailPage />} />
        <Route path="/games/:id" element={<GamePage />} />
      </Route>

      <Route path="*" element={<Navigate to="/lobbies" replace />} />
    </Routes>
  )
}

export default App
