import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth/auth'

export function RequireAuth() {
  const { user, loading } = useAuth()
  const loc = useLocation()
  if (loading) return null
  if (!user) return <Navigate to="/login" state={{ from: loc.pathname }} replace />
  return <Outlet />
}


