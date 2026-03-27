import { useCallback, useEffect, useRef, useState } from 'react'
import type { WsClient } from '../ws/wsClient'

function LocalVideo({ stream }: { stream: MediaStream }) {
  const ref = useRef<HTMLVideoElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.srcObject = stream
    return () => {
      el.srcObject = null
    }
  }, [stream])
  return (
    <div style={{ flex: '0 0 auto' }}>
      <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4, opacity: 0.85 }}>You</div>
      <video
        ref={ref}
        autoPlay
        playsInline
        muted
        style={{
          width: 160,
          height: 120,
          objectFit: 'cover',
          borderRadius: 12,
          background: '#0f172a',
        }}
      />
    </div>
  )
}

function RemoteVideo({ userId, stream }: { userId: number; stream: MediaStream }) {
  const ref = useRef<HTMLVideoElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.srcObject = stream
    return () => {
      el.srcObject = null
    }
  }, [stream])
  return (
    <div style={{ flex: '0 0 auto' }}>
      <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4, opacity: 0.85 }}>Peer {userId}</div>
      <video
        ref={ref}
        autoPlay
        playsInline
        style={{
          width: 160,
          height: 120,
          objectFit: 'cover',
          borderRadius: 12,
          background: '#0f172a',
        }}
      />
    </div>
  )
}

type VideoChatProps = {
  ws: WsClient
  myUserId: number
  peerUserIds: number[]
  onError?: (message: string) => void
}

export function VideoChat({ ws, myUserId, peerUserIds, onError }: VideoChatProps) {
  const [localStream, setLocalStream] = useState<MediaStream | null>(null)
  const [remoteStreams, setRemoteStreams] = useState<Map<number, MediaStream>>(new Map())
  const [muted, setMuted] = useState(false)
  const [videoOff, setVideoOff] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pcsRef = useRef<Map<number, RTCPeerConnection>>(new Map())
  const pendingIceRef = useRef<Map<number, RTCIceCandidateInit[]>>(new Map())
  const offeredRef = useRef<Set<number>>(new Set())

  const reportError = useCallback(
    (msg: string) => {
      setError(msg)
      onError?.(msg)
    },
    [onError]
  )

  // Get user media on mount
  useEffect(() => {
    let stream: MediaStream | null = null
    let cancelled = false
    navigator.mediaDevices
      .getUserMedia({ video: true, audio: true })
      .then((s) => {
        if (!cancelled) {
          stream = s
          setLocalStream(s)
        } else {
          s.getTracks().forEach((t) => t.stop())
        }
      })
      .catch((err) => {
        if (!cancelled) {
          reportError(err instanceof Error ? err.message : 'Could not access camera/microphone')
        }
      })
    return () => {
      cancelled = true
      if (stream) {
        stream.getTracks().forEach((t) => t.stop())
      }
    }
  }, [reportError])

  // Mute / video off apply to local stream
  useEffect(() => {
    if (!localStream) return
    localStream.getAudioTracks().forEach((t) => {
      t.enabled = !muted
    })
  }, [localStream, muted])
  useEffect(() => {
    if (!localStream) return
    localStream.getVideoTracks().forEach((t) => {
      t.enabled = !videoOff
    })
  }, [localStream, videoOff])

  const getOrCreatePc = useCallback(
    (peerId: number): RTCPeerConnection => {
      let pc = pcsRef.current.get(peerId)
      if (pc) return pc
      pc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
      })
      pcsRef.current.set(peerId, pc)

      pc.ontrack = (e) => {
        const stream = e.streams[0] ?? new MediaStream(e.track ? [e.track] : [])
        setRemoteStreams((prev) => new Map(prev).set(peerId, stream))
      }
      pc.onicecandidate = (e) => {
        if (e.candidate && e.candidate.candidate) {
          ws.send('webrtc_ice', {
            to_user_id: peerId,
            candidate: e.candidate.toJSON(),
          })
        }
      }
      pc.onconnectionstatechange = () => {
        if (pc.connectionState === 'failed' || pc.connectionState === 'closed') {
          setRemoteStreams((prev) => {
            const next = new Map(prev)
            next.delete(peerId)
            return next
          })
        }
      }

      if (localStream) {
        localStream.getTracks().forEach((t) => pc.addTrack(t, localStream))
      }
      return pc
    },
    [ws, localStream]
  )

  // Create offers for peers where we are the offerer (lower user id offers), once per peer
  useEffect(() => {
    if (!localStream || peerUserIds.length === 0) return
    const offerers = peerUserIds.filter((pid) => myUserId < pid && !offeredRef.current.has(pid))
    offerers.forEach((peerId) => {
      offeredRef.current.add(peerId)
      void (async () => {
        try {
          const pc = getOrCreatePc(peerId)
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          ws.send('webrtc_offer', {
            to_user_id: peerId,
            sdp: pc.localDescription?.toJSON(),
          })
        } catch (err) {
          offeredRef.current.delete(peerId)
          reportError(err instanceof Error ? err.message : 'Failed to create offer')
        }
      })()
    })
  }, [localStream, myUserId, peerUserIds, getOrCreatePc, ws, reportError])

  // Handle incoming signaling
  useEffect(() => {
    const unsubOffer = ws.on('webrtc_offer', async (payload: unknown) => {
      const p = payload as { from_user_id?: number; sdp?: RTCSessionDescriptionInit }
      const from = typeof p?.from_user_id === 'number' ? p.from_user_id : 0
      const sdp = p?.sdp
      if (!from || !sdp || from === myUserId) return
      try {
        const pc = getOrCreatePc(from)
        await pc.setRemoteDescription(new RTCSessionDescription(sdp))
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        ws.send('webrtc_answer', {
          to_user_id: from,
          sdp: pc.localDescription?.toJSON(),
        })
        // Drain any pending ICE candidates that arrived before remote description
        const pending = pendingIceRef.current.get(from) ?? []
        pendingIceRef.current.delete(from)
        for (const c of pending) {
          await pc.addIceCandidate(new RTCIceCandidate(c)).catch(() => {})
        }
      } catch (err) {
        reportError(err instanceof Error ? err.message : 'Failed to handle offer')
      }
    })

    const unsubAnswer = ws.on('webrtc_answer', async (payload: unknown) => {
      const p = payload as { from_user_id?: number; sdp?: RTCSessionDescriptionInit }
      const from = typeof p?.from_user_id === 'number' ? p.from_user_id : 0
      const sdp = p?.sdp
      if (!from || !sdp) return
      const pc = pcsRef.current.get(from)
      if (!pc) return
      try {
        await pc.setRemoteDescription(new RTCSessionDescription(sdp))
        const pending = pendingIceRef.current.get(from) ?? []
        pendingIceRef.current.delete(from)
        for (const c of pending) {
          await pc.addIceCandidate(new RTCIceCandidate(c)).catch(() => {})
        }
      } catch (err) {
        reportError(err instanceof Error ? err.message : 'Failed to set remote description')
      }
    })

    const unsubIce = ws.on('webrtc_ice', async (payload: unknown) => {
      const p = payload as { from_user_id?: number; candidate?: RTCIceCandidateInit }
      const from = typeof p?.from_user_id === 'number' ? p.from_user_id : 0
      const candidate = p?.candidate
      if (!from || !candidate) return
      const pc = pcsRef.current.get(from)
      if (!pc) {
        const pending = pendingIceRef.current.get(from) ?? []
        pending.push(candidate)
        pendingIceRef.current.set(from, pending)
        return
      }
      if (pc.remoteDescription) {
        await pc.addIceCandidate(new RTCIceCandidate(candidate)).catch(() => {})
      } else {
        const pending = pendingIceRef.current.get(from) ?? []
        pending.push(candidate)
        pendingIceRef.current.set(from, pending)
      }
    })

    return () => {
      unsubOffer()
      unsubAnswer()
      unsubIce()
    }
  }, [ws, myUserId, getOrCreatePc, reportError])

  // Cleanup peer connections when peers change or on unmount
  useEffect(() => {
    const currentPeers = new Set(peerUserIds)
    pcsRef.current.forEach((pc, peerId) => {
      if (!currentPeers.has(peerId)) {
        pc.close()
        pcsRef.current.delete(peerId)
        pendingIceRef.current.delete(peerId)
        offeredRef.current.delete(peerId)
        setRemoteStreams((prev) => {
          const next = new Map(prev)
          next.delete(peerId)
          return next
        })
      }
    })
    return () => {
      pcsRef.current.forEach((pc) => pc.close())
      pcsRef.current.clear()
      pendingIceRef.current.clear()
      offeredRef.current.clear()
      setRemoteStreams(new Map())
    }
  }, [peerUserIds])

  if (error) {
    return (
      <div style={{ padding: 12, background: '#fef2f2', borderRadius: 12, color: '#b91c1c' }}>
        Video chat: {error}
      </div>
    )
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
        padding: 16,
        background: 'rgba(15,23,42,0.06)',
        borderRadius: 16,
        border: '1px solid var(--color-border)',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <span style={{ fontWeight: 700, opacity: 0.9 }}>Video chat</span>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            onClick={() => setMuted((m) => !m)}
            title={muted ? 'Unmute' : 'Mute'}
            style={{
              padding: '6px 12px',
              borderRadius: 8,
              border: '1px solid var(--color-border)',
              background: muted ? '#fecaca' : 'var(--color-bg-button)',
            }}
          >
            {muted ? '🔇 Unmute' : '🔊 Mute'}
          </button>
          <button
            type="button"
            onClick={() => setVideoOff((v) => !v)}
            title={videoOff ? 'Turn video on' : 'Turn video off'}
            style={{
              padding: '6px 12px',
              borderRadius: 8,
              border: '1px solid var(--color-border)',
              background: videoOff ? '#fecaca' : 'var(--color-bg-button)',
            }}
          >
            {videoOff ? '📷 Video on' : '📷 Video off'}
          </button>
        </div>
      </div>
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        {localStream && (
          <LocalVideo stream={localStream} />
        )}
        {Array.from(remoteStreams.entries()).map(([userId, stream]) => (
          <RemoteVideo key={userId} userId={userId} stream={stream} />
        ))}
      </div>
      {!localStream && (
        <div style={{ opacity: 0.8, fontSize: 14 }}>Requesting camera and microphone…</div>
      )}
    </div>
  )
}
