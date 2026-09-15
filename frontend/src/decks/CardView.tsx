import { useState } from 'react'
import type React from 'react'
import type { Card } from '../api/types'
import { CARD_H, CARD_R, CARD_W, cardToString, rankLabel, suitSymbol } from './cardLabels'
import { useDeck } from './DeckProvider'
import { DEFAULT_DECK_THEME, faceImageUrlFor, suitColorFor, type DeckTheme } from './deckTheme'

/** Vector card face, used for built-in decks and as the image fallback. */
function DrawnFace({ card, theme, selected }: { card: Card; theme: DeckTheme; selected?: boolean }) {
  const rank = rankLabel(card.rank)
  const suit = suitSymbol(card.suit)
  const color = selected ? 'white' : suitColorFor(theme, card.suit)

  return (
    <>
      <div
        style={{
          position: 'absolute',
          top: 7,
          left: 8,
          fontSize: 14,
          fontWeight: 700,
          lineHeight: 1,
          color,
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
          color,
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
          color,
        }}
      >
        {rank}
      </div>
    </>
  )
}

/**
 * Renders a deck's card-face image, falling back to the drawn face if the image
 * cannot be loaded. Mount this with `key={src}` so a new URL retries cleanly.
 */
function CardFaceImage({
  src,
  card,
  theme,
  selected,
}: {
  src: string
  card: Card
  theme: DeckTheme
  selected?: boolean
}) {
  const [failed, setFailed] = useState(false)
  if (failed) return <DrawnFace card={card} theme={theme} selected={selected} />
  return (
    <img
      src={src}
      alt={cardToString(card)}
      onError={() => setFailed(true)}
      style={{
        width: '100%',
        height: '100%',
        objectFit: 'cover',
        borderRadius: CARD_R,
        display: 'block',
        // Tint the selected state without hiding the artwork.
        opacity: selected ? 0.75 : 1,
      }}
    />
  )
}

/**
 * A single card face. When the active deck supplies a face-image template the
 * image is used; if it fails to load we fall back to the drawn face so a broken
 * CDN never leaves the table blank.
 */
export function CardIcon({
  card,
  selected,
  disabled,
  muted,
  onClick,
  title,
  theme: themeOverride,
}: {
  card: Card
  selected?: boolean
  disabled?: boolean
  muted?: boolean
  onClick?: () => void
  title?: string
  /** Overrides the active deck; used by deck previews. */
  theme?: DeckTheme
}) {
  const deck = useDeck()
  const theme = themeOverride ?? deck.theme
  const interactive = !!onClick && !disabled

  const faceUrl = faceImageUrlFor(theme, card)

  const outerStyle: React.CSSProperties = {
    width: CARD_W,
    height: CARD_H,
    padding: 0,
    borderRadius: CARD_R,
    border: `1px solid ${theme.borderColor}`,
    background: selected ? '#2563eb' : '#ffffff',
    cursor: interactive ? 'pointer' : 'default',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: selected ? '0 0 0 2px rgba(37,99,235,0.25)' : undefined,
    opacity: muted ? 0.55 : 1,
    overflow: 'hidden',
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
      {faceUrl ? (
        // Keyed by URL so switching decks retries a previously failed image.
        <CardFaceImage key={faceUrl} src={faceUrl} card={card} theme={theme} selected={selected} />
      ) : (
        <DrawnFace card={card} theme={theme} selected={selected} />
      )}
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

/** The face-down card back, themed by the active deck. */
export function CardBack({ title, theme: themeOverride }: { title?: string; theme?: DeckTheme }) {
  const deck = useDeck()
  const theme = themeOverride ?? deck.theme
  const backUrl = theme.backImageUrl

  const base: React.CSSProperties = {
    // Match CardIcon / ActionCard sizing so all cards feel consistent on the table.
    width: CARD_W,
    height: CARD_H,
    borderRadius: CARD_R,
    border: `1px solid ${theme.borderColor}`,
    boxShadow: '0 1px 2px rgba(0,0,0,0.12)',
    overflow: 'hidden',
  }

  // Fallback: the classic woven pattern, tinted with the deck's suit colors.
  const tint = theme.blackSuitColor || DEFAULT_DECK_THEME.blackSuitColor
  const tint2 = theme.redSuitColor || DEFAULT_DECK_THEME.redSuitColor
  const patterned = (
    <div
      title={title}
      style={{
        ...base,
        background: `repeating-linear-gradient(45deg, ${tint} 0px, ${tint} 6px, ${tint2} 6px, ${tint2} 12px)`,
      }}
    />
  )

  if (!backUrl) return patterned
  // Keyed by URL so switching decks retries a previously failed image.
  return <CardBackImage key={backUrl} src={backUrl} title={title} style={base} fallback={patterned} />
}

/** Card back image with a fallback for load failures. */
function CardBackImage({
  src,
  title,
  style,
  fallback,
}: {
  src: string
  title?: string
  style: React.CSSProperties
  fallback: React.ReactNode
}) {
  const [failed, setFailed] = useState(false)
  if (failed) return <>{fallback}</>
  return (
    <div title={title} style={style}>
      <img
        src={src}
        alt=""
        onError={() => setFailed(true)}
        style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }}
      />
    </div>
  )
}

/** A card-shaped action button, sized to match the cards on the table. */
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
