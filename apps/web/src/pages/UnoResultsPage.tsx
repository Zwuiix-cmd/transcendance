
import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import type { PlayerResult } from '../types'

export function UnoResultsPage() {
  const location = useLocation()
  const navigate = useNavigate()

  const results = location.state?.results as PlayerResult[] | undefined

  if (!results) {
    return (
      <main className="main-content">
        <div className="page-stack">
          <section className="card">
            <span className="eyebrow">UNO</span>
            <h1>Results unavailable</h1>
            <p className="page-description">
              No game data was found.
            </p>
            <button
              className="button button-primary"
              onClick={() => navigate('/dashboard', { replace: true })}
            >
              Return to dashboard
            </button>
          </section>
        </div>
      </main>
    )
  }

  const winner = results.find(player => player.winner)

  return (
    <main className="main-content">
      <div className="page-stack">

        <header className="page-header">
          <div>
            <span className="eyebrow">UNO · END OF GAME</span>
            <h1>Results</h1>
            <p className="page-description">
              The game is over. Here is the final ranking of the players.
            </p>
          </div>
        </header>

        {winner && (
          <section className="card">
            <span className="eyebrow">WINNER</span>
            <h2 style={{ fontSize: '1.6rem', marginTop: 12 }}>
              {winner.name}
            </h2>
            <p className="muted" style={{ marginTop: 8 }}>
              {winner.pointsEarned} points earned
            </p>
          </section>
        )}

        <section className="card">
          <div className="page-header">
            <div>
              <span className="eyebrow">RANKING</span>
              <h2 style={{ marginTop: 8 }}>Final standings</h2>
            </div>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {results.map(result => (
              <div
                key={result.playerId}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 16,
                  padding: 16,
                  border: '1px solid var(--line)',
                  borderRadius: 8,
                  background: result.winner
                    ? 'var(--accent-soft)'
                    : 'var(--surface-muted)',
                }}
              >
                <div>
                  <strong>
                    <span style={{ color: 'var(--accent)', marginRight: 10 }}>
                      #{result.rank}
                    </span>
                    {result.name}
                  </strong>
                  <p className="muted" style={{ marginTop: 6, fontSize: '.8rem' }}>
                    {result.cardsRemaining} remaining cards
                  </p>
                </div>

                <div style={{ textAlign: 'right' }}>
                  <strong>{result.pointsEarned} pts</strong>
                  <p className="muted" style={{ marginTop: 6, fontSize: '.75rem' }}>
                    Card values : {result.handPoints}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </section>

        <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 10 }}>
          <button
            className="button button-primary"
            onClick={() => navigate('/dashboard', { replace: true })}
          >
            Return to dashboard
          </button>
        </div>

      </div>
    </main>
  )
}
