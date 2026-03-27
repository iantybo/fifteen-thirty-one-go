import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { WagmiProvider } from 'wagmi'
import './index.css'
import App from './App.tsx'
import { AuthProvider } from './auth/auth'
import { queryClient, wagmiConfig } from './lib/wagmi'

const rootEl = document.getElementById('root')
if (!rootEl) {
  throw new Error('Failed to initialize app: expected a DOM element with id="root".')
}

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <WagmiProvider config={wagmiConfig}>
        <AuthProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </AuthProvider>
      </WagmiProvider>
    </QueryClientProvider>
  </StrictMode>,
)
