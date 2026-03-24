import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { mainnet } from 'viem/chains'
import { useAccount, useChainId, useConnect, useSignMessage, useSwitchChain } from 'wagmi'
import { api } from '../api/client'
import { useAuth } from '../auth/auth'

type WalletLoginProps = {
  /** Where to navigate after successful wallet auth (default `/lobbies`). */
  navigateTo?: string
}

function isUserRejectedError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  const o = err as { code?: number; message?: string }
  if (o.code === 4001) return true
  if (typeof o.message === 'string' && /user rejected|denied|cancel/i.test(o.message)) return true
  return false
}

export function WalletLogin({ navigateTo = '/lobbies' }: WalletLoginProps) {
  const nav = useNavigate()
  const { setAuth } = useAuth()
  const { address, isConnected } = useAccount()
  const chainId = useChainId()
  const { connectors, connectAsync, isPending: connectPending } = useConnect()
  const { switchChainAsync } = useSwitchChain()
  const { signMessageAsync, isPending: signPending } = useSignMessage()

  const [err, setErr] = useState<string | null>(null)
  const busy = connectPending || signPending

  function pickConnector() {
    return (
      connectors.find((c) => c.id === 'injected') ??
      connectors.find((c) => c.id === 'walletConnect') ??
      connectors[0]
    )
  }

  async function ensureEthereumAddress(): Promise<`0x${string}`> {
    if (address) return address
    const connector = pickConnector()
    if (!connector) {
      throw new Error(
        'No wallet connector available. Install MetaMask or another browser wallet, or set VITE_WALLETCONNECT_PROJECT_ID for WalletConnect.',
      )
    }
    const { accounts } = await connectAsync({
      connector,
      chainId: mainnet.id,
    })
    const first = accounts?.[0]
    if (!first) throw new Error('Wallet did not return an address.')
    return first
  }

  async function ensureMainnet() {
    if (!switchChainAsync) {
      if (chainId !== mainnet.id) {
        throw new Error('Wrong network: switch your wallet to Ethereum Mainnet.')
      }
      return
    }
    if (chainId !== mainnet.id) {
      await switchChainAsync({ chainId: mainnet.id })
    }
  }

  async function onWalletAuth() {
    setErr(null)
    try {
      const walletAddress = await ensureEthereumAddress()
      await ensureMainnet()

      const { challenge } = await api.walletChallenge({ wallet_address: walletAddress })
      const signature = await signMessageAsync({ message: challenge })
      const res = await api.walletVerify({
        wallet_address: walletAddress,
        challenge,
        signature,
      })
      setAuth(res.user, { viaWallet: true })
      nav(navigateTo, { replace: true })
    } catch (e: unknown) {
      if (isUserRejectedError(e)) {
        setErr('Request was rejected in your wallet.')
        return
      }
      if (e instanceof Error && e.message.includes('Chain mismatch')) {
        setErr('Wrong network: switch to Ethereum Mainnet in your wallet.')
        return
      }
      setErr(e instanceof Error ? e.message : 'Wallet sign-in failed')
    }
  }

  const hasInjected = connectors.some((c) => c.id === 'injected' || c.type === 'injected')
  const hasWC = connectors.some((c) => c.id === 'walletConnect')

  return (
    <div style={{ marginTop: 16 }}>
      <button type="button" disabled={busy} onClick={() => void onWalletAuth()}>
        {busy ? 'Working…' : 'Continue with Ethereum wallet'}
      </button>
      {isConnected && address && chainId !== mainnet.id && (
        <p style={{ color: 'darkorange', marginTop: 8, fontSize: 14 }}>
          Connected on the wrong network — you will be asked to switch to Ethereum Mainnet.
        </p>
      )}
      {!hasInjected && !hasWC && (
        <p style={{ color: 'crimson', marginTop: 8, fontSize: 14 }}>
          No wallet connectors available. Install a browser extension wallet or configure WalletConnect.
        </p>
      )}
      {err && (
        <div style={{ color: 'crimson', marginTop: 8, fontSize: 14 }} role="alert">
          {err}
        </div>
      )}
    </div>
  )
}
