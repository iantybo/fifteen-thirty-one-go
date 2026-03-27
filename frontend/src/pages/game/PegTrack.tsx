import type React from 'react'
import type { GameSnapshot } from '../../api/types'
import type { PegBoardTheme } from './pegBoardThemes'

export function PegTrack({
  players,
  scores,
  theme,
  headerAction,
}: {
  players: GameSnapshot['players']
  scores: number[] | undefined
  theme: PegBoardTheme
  headerAction?: React.ReactNode
}) {
  const max = 121
  const endPad = 18
  const sorted = players.slice().sort((a, b) => a.position - b.position)
  const colors = theme.pegColors

  const posPct = (v: number) => `${Math.max(0, Math.min(max, v)) / max * 100}%`

  return (
    <div
      style={{
        marginTop: 10,
        padding: 10,
        border: `1px solid ${theme.containerBorder}`,
        borderRadius: 10,
        background: theme.containerBg,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8, gap: 8 }}>
        <div style={{ fontWeight: 700 }}>Peg board</div>
        {headerAction}
      </div>
      <div
        style={{
          position: 'relative',
          height: 54,
          borderRadius: 999,
          background: theme.trackBg,
          border: `1px solid ${theme.trackBorder}`,
          overflow: 'hidden',
        }}
      >
        {/* Endcaps so 0/121 don't feel cramped */}
        <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: endPad, background: theme.startCapBg }} />
        <div
          style={{
            position: 'absolute',
            right: 0,
            top: 0,
            bottom: 0,
            width: endPad,
            background: theme.endCapBg,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 900,
            color: theme.endCapTextColor,
            textShadow: theme.endCapTextShadow,
            fontSize: 12,
          }}
          title="121"
        >
          121
        </div>

        {/* Inner lane where ticks/pegs are positioned (gives padding at the ends) */}
        <div style={{ position: 'absolute', left: endPad, right: endPad, top: 0, bottom: 0 }}>
          {/* Peg holes (two rows) */}
          {Array.from({ length: max + 1 }).map((_, v) => {
            const isTen = v % 10 === 0
            const isFive = v % 5 === 0
            const size = isTen ? 5 : isFive ? 4 : 3
            const alpha = isTen ? 0.28 : isFive ? 0.18 : 0.12
            const common: React.CSSProperties = {
              position: 'absolute',
              left: posPct(v),
              width: size,
              height: size,
              borderRadius: 999,
              transform: 'translateX(-50%)',
              background: `rgba(${theme.holeColor}, ${alpha})`,
              boxShadow: theme.holeShadow,
              pointerEvents: 'none',
            }
            return (
              <div key={`hole:${v}`}>
                <div style={{ ...common, top: 22 - size / 2 }} />
                <div style={{ ...common, top: 42 - size / 2 }} />
              </div>
            )
          })}

          {/* tick marks */}
          {Array.from({ length: Math.floor(max / 5) + 1 }).map((_, i) => {
            const v = i * 5
            const isTen = v % 10 === 0
            return (
              <div
                key={`tick:${v}`}
                style={{
                  position: 'absolute',
                  left: posPct(v),
                  top: 0,
                  bottom: 0,
                  width: 1,
                  background: isTen ? theme.tickMajor : theme.tickMinor,
                  opacity: isTen ? 1 : 0.75,
                }}
                title={String(v)}
              />
            )
          })}

          {sorted.map((p, idx) => {
            const s = scores?.[p.position] ?? 0
            const c = colors[idx % colors.length]
            return (
              <div
                key={`peg:${p.position}`}
                style={{
                  position: 'absolute',
                  left: posPct(s),
                  top: 14 + (idx % 2) * 20,
                  transform: 'translateX(-50%)',
                  transition: 'left 420ms cubic-bezier(0.2, 0.8, 0.2, 1)',
                }}
              >
                <div style={{ position: 'relative' }}>
                  <div
                    style={{
                      width: 16,
                      height: 16,
                      borderRadius: 999,
                      background: c,
                      border: `2px solid ${theme.pegBorder}`,
                      boxShadow: theme.pegShadow,
                    }}
                    title={`P${p.position}: ${s}`}
                  />
                  <div
                    style={{
                      position: 'absolute',
                      top: -2,
                      left: 20,
                      fontSize: 11,
                      fontWeight: 800,
                      color: theme.scoreBadgeColor,
                      background: theme.scoreBadgeBg,
                      border: `1px solid ${theme.scoreBadgeBorder}`,
                      borderRadius: 999,
                      padding: '0 6px',
                    }}
                  >
                    {s}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
      {/* Vertical ruler labels */}
      <div style={{ marginTop: 4, position: 'relative', height: 28, fontSize: 10, opacity: 0.85 }}>
        <div style={{ position: 'absolute', left: endPad, right: endPad, top: 0, bottom: 0 }}>
          {Array.from({ length: Math.floor(max / 5) + 1 }).map((_, i) => {
            const v = i * 5
            const isTen = v % 10 === 0
            return (
              <div
                key={`label:${v}`}
                style={{
                  position: 'absolute',
                  left: posPct(v),
                  transform: 'translateX(-50%)',
                  writingMode: 'vertical-rl',
                  textOrientation: 'mixed',
                  lineHeight: 1,
                  fontWeight: isTen ? 800 : 600,
                  color: isTen ? theme.labelMajor : theme.labelMinor,
                  pointerEvents: 'none',
                }}
              >
                {v}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
