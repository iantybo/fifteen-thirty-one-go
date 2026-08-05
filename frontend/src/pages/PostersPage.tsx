import { useCallback, useEffect, useState } from 'react'

/** A movie poster shown in the carousel, with a human-authored annotation. */
interface Poster {
  src: string
  title: string
  /** Alt text / description of what the poster is claimed to depict. */
  alt: string
  annotation: string
}

const POSTERS: readonly Poster[] = [
  {
    src: '/posters/feel_good_baseball_dog_movie_poster.png',
    title: 'Home Run',
    alt: 'Movie poster for a feel-good baseball film: a golden retriever wearing a baseball cap and jersey sits on the pitch at sunset, holding a baseball in its mouth.',
    annotation:
      'A feel-good baseball movie poster. A golden retriever in a cap and jersey sits center-frame on the field, holding a baseball, under the tagline "Sometimes, life throws you a curveball."',
  },
  {
    src: '/posters/witches_cat_movie_poster.png',
    title: 'A Witches Move',
    alt: 'Movie poster for a fantasy film: a black cat wearing a purple witch hat and cloak sits beside a glowing crystal ball and candle, with a full moon behind it.',
    annotation:
      'A fantasy movie poster. A black cat dressed as a witch — pointed hat, purple cloak, pentacle charm — sits before a crystal ball and moonlit window under the tagline "Some magic never fades."',
  },
  {
    src: '/posters/romance_movie_poster.png',
    title: "Destiny's Sparkle",
    alt: 'Romance movie poster: a couple embracing at sunset on a beach with a heart in the sky.',
    annotation:
      'A romance movie poster. Two lovers share a tender moment under the tagline "True love. Universe. Vibes."',
  },
]

export function PostersPage() {
  const [index, setIndex] = useState(0)
  const count = POSTERS.length

  const prev = useCallback(() => {
    setIndex((i) => (i - 1 + count) % count)
  }, [count])

  const next = useCallback(() => {
    setIndex((i) => (i + 1) % count)
  }, [count])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'ArrowLeft') prev()
      if (e.key === 'ArrowRight') next()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
    }
  }, [prev, next])

  const current = POSTERS[index]

  return (
    <div
      style={{
        minHeight: '100vh',
        background: 'var(--color-lobby-bg)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px 16px',
        boxSizing: 'border-box',
      }}
    >
      <h1 style={{ color: '#fff', marginTop: 0, textAlign: 'center' }}>Coming Soon</h1>

      <div
        style={{
          background: 'var(--color-lobby-card)',
          borderRadius: 16,
          padding: 24,
          maxWidth: 480,
          width: '100%',
          boxShadow: '0 10px 30px rgba(0,0,0,0.25)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 16,
          }}
        >
          <button type="button" onClick={prev} aria-label="Previous poster">
            ‹
          </button>

          <img
            key={current.src}
            src={current.src}
            alt={current.alt}
            style={{
              flex: 1,
              minWidth: 0,
              width: '100%',
              height: 'auto',
              borderRadius: 8,
              display: 'block',
            }}
          />

          <button type="button" onClick={next} aria-label="Next poster">
            ›
          </button>
        </div>

        <h2 style={{ marginBottom: 4 }}>{current.title}</h2>
        <p style={{ marginTop: 0 }}>{current.annotation}</p>

        <div
          style={{
            display: 'flex',
            gap: 8,
            justifyContent: 'center',
            marginTop: 8,
          }}
          role="tablist"
          aria-label="Poster selection"
        >
          {POSTERS.map((poster, i) => (
            <button
              key={poster.src}
              type="button"
              role="tab"
              aria-selected={i === index}
              aria-label={`Show poster ${i + 1}: ${poster.title}`}
              onClick={() => setIndex(i)}
              style={{
                width: 12,
                height: 12,
                padding: 0,
                borderRadius: '50%',
                background: i === index ? 'var(--color-primary)' : 'var(--color-border)',
              }}
            />
          ))}
        </div>
      </div>
    </div>
  )
}
