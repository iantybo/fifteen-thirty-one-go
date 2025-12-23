import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth/auth'

export function RequireAuth() {
  const { token } = useAuth()
  const loc = useLocation()
  if (!token) return <Navigate to="/login" state={{ from: loc.pathname }} replace />
  return <Outlet />
}


