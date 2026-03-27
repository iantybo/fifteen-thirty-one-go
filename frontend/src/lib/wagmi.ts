import { QueryClient } from '@tanstack/react-query'
import { createConfig, http } from 'wagmi'
import { mainnet } from 'viem/chains'
import { injected, walletConnect } from '@wagmi/connectors'

const walletConnectProjectId = (import.meta.env.VITE_WALLETCONNECT_PROJECT_ID as string | undefined)?.trim()

const connectors = [
  injected(),
  ...(walletConnectProjectId
    ? [
        walletConnect({
          projectId: walletConnectProjectId,
        }),
      ]
    : []),
]

export const wagmiConfig = createConfig({
  chains: [mainnet],
  transports: {
    [mainnet.id]: http(),
  },
  connectors,
})

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
    },
  },
})
