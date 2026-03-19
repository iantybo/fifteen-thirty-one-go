import type React from 'react'
import type { Card } from '../../api/types'
import { CARD_W, CARD_H, CARD_R, rankLabel, suitSymbol, suitColor } from './cardUtils'

export function CardIcon({
  card,
  selected,
  disabled,
  muted,
  onClick,
  title,
}: {
  card: Card
  selected?: boolean
  disabled?: boolean
  muted?: boolean
  onClick?: () => void
  title?: string
}) {
  const rank = rankLabel(card.rank)
  const suit = suitSymbol(card.suit)
  const color = suitColor(card.suit)
  const interactive = !!onClick && !disabled

  const outerStyle: React.CSSProperties = {
    width: CARD_W,
    height: CARD_H,
    padding: 0,
    borderRadius: CARD_R,
    border: '1px solid #cbd5e1',
    background: selected ? '#2563eb' : '#ffffff',
    cursor: interactive ? 'pointer' : 'default',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: selected ? '0 0 0 2px rgba(37,99,235,0.25)' : undefined,
    opacity: muted ? 0.55 : 1,
  }

  const inner = (
    <div
      style={{
        width: '100%',
        height: '100%',
        position: 'relative',
        borderRadius: CARD_R,
        background: selected ? '#2563eb' : '#ffffff',
      }}
    >
      <div
        style={{
          position: 'absolute',
          top: 7,
          left: 8,
          fontSize: 14,
          fontWeight: 700,
          lineHeight: 1,
          color: selected ? 'white' : color,
        }}
      >
        {rank}
      </div>
      <div
        style={{
          position: 'absolute',
          top: 30,
          left: 0,
          right: 0,
          textAlign: 'center',
          fontSize: 36,
          lineHeight: 1,
          color: selected ? 'white' : color,
        }}
      >
        {suit}
      </div>
      <div
        style={{
          position: 'absolute',
          bottom: 7,
          right: 8,
          fontSize: 14,
          fontWeight: 700,
          lineHeight: 1,
          transform: 'rotate(180deg)',
          color: selected ? 'white' : color,
        }}
      >
        {rank}
      </div>
    </div>
  )

  if (!interactive) {
    return (
      <div aria-disabled={disabled ? true : undefined} title={title} style={outerStyle}>
        {inner}
      </div>
    )
  }

  return (
    <button type="button" onClick={onClick} title={title} style={outerStyle}>
      {inner}
    </button>
  )
}

export function ActionCard({
  label,
  disabled,
  onClick,
  title,
  accent,
}: {
  label: string
  disabled?: boolean
  onClick?: () => void
  title?: string
  accent?: 'primary' | 'danger'
}) {
  const bg = accent === 'primary' ? '#2563eb' : accent === 'danger' ? '#dc2626' : '#ffffff'
  const fg = accent ? '#ffffff' : '#0f172a'
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      title={title}
      style={{
        width: CARD_W,
        height: CARD_H,
        padding: 0,
        borderRadius: CARD_R,
        border: '1px solid #cbd5e1',
        background: bg,
        cursor: disabled ? 'not-allowed' : 'pointer',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontWeight: 900,
        letterSpacing: 0.8,
        color: fg,
        opacity: disabled ? 0.6 : 1,
      }}
    >
      {label}
    </button>
  )
}

export function CardBack({ title }: { title?: string }) {
  return (
    <div
      title={title}
      style={{
        // Match CardIcon / ActionCard sizing so all cards feel consistent on the table.
        width: CARD_W,
        height: CARD_H,
        borderRadius: CARD_R,
        border: '1px solid #cbd5e1',
        background:
          'repeating-linear-gradient(45deg, #1d4ed8 0px, #1d4ed8 6px, #2563eb 6px, #2563eb 12px)',
        boxShadow: '0 1px 2px rgba(0,0,0,0.12)',
      }}
    />
  )
}
