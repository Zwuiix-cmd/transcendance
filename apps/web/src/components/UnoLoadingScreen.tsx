import { useEffect, useState } from 'react'
import { UserAvatar } from './ui'
import { botDifficultyLabel } from '../gameSettings'
import type { BotDifficulty } from '../types'

export type LoadingSeat = { userId: string; username: string; avatarUrl?: string | null; bot: boolean }

const STAGES: Record<string, string> = {
  joining: 'Joining the table',
  connecting: 'Connecting to the table',
  lighting: 'Lighting the room',
  table: 'Setting the table',
  cards: 'Shuffling the deck',
  players: 'Seating the players',
  ready: 'Dealing the cards',
}

const TIPS = [
  'Match the color, the number or the symbol. A Wild always fits.',
  'Press U before playing your second-to-last card, or risk two penalty cards.',
  'Draw one card when stuck. If it fits, play it right away.',
  'A Wild +4 can be challenged: press C when you smell a bluff.',
  'With two players, Reverse acts like a Skip and gives you another turn.',
  'Click the hand of a player who forgot UNO to catch them.',
  'Right-click the table to look around, use the crosshair to pick cards.',
]

const FAN_CARDS = ['R7', 'G4', 'W4', 'B9', 'Y2']

export function UnoLoadingScreen({ stage, ratio, seats, difficulty, spectator, leaving, stalled, onSkip }: {
  stage: string
  ratio: number
  seats: LoadingSeat[]
  difficulty?: BotDifficulty
  spectator: boolean
  leaving: boolean
  stalled: boolean
  onSkip: () => void
}) {
  const [tip, setTip] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTip((current) => (current + 1) % TIPS.length), 3400)
    return () => window.clearInterval(timer)
  }, [])
  const percent = Math.round(Math.min(Math.max(ratio, 0), 1) * 100)

  return <div className={`uno-loading${leaving ? ' is-leaving' : ''}`} role="status" aria-live="polite" aria-label="Loading the table">
    <div className="uno-loading-aurora" aria-hidden="true"><span /><span /><span /><span /></div>
    <div className="uno-loading-fan" aria-hidden="true">
      {FAN_CARDS.map((face, index) => <img key={face} className="uno-loading-card" style={{ ['--i' as string]: index - (FAN_CARDS.length - 1) / 2 }} src={`/files/uno/cards/${face}.png`} alt="" draggable={false} />)}
    </div>
    <div className="uno-loading-body">
      <span className="uno-game-overlay-eyebrow">Uno Ranked</span>
      <h1 className="uno-loading-title">{spectator ? 'Taking a seat in the stands' : STAGES[stage] ?? 'Preparing the table'}<span className="uno-loading-dots"><i /><i /><i /></span></h1>
      <div className="uno-loading-bar" aria-hidden="true"><div className="uno-loading-bar-fill" style={{ width: `${percent}%` }} /></div>
      <div className="uno-loading-meta"><span>{STAGES[stage] ?? 'Preparing the table'}</span><strong>{percent}%</strong></div>
      {seats.length > 0 ? <ul className="uno-loading-seats">{seats.map((seat, index) => <li key={seat.userId} style={{ ['--d' as string]: `${index * 90}ms` }}><UserAvatar username={seat.username} avatarUrl={seat.avatarUrl} className="uno-loading-avatar" /><span>{seat.username}</span>{seat.bot ? <small>{botDifficultyLabel(difficulty)} bot</small> : null}</li>)}</ul> : null}
      <p className="uno-loading-tip" key={tip}>{TIPS[tip]}</p>
      {stalled ? <div className="uno-loading-stalled"><span>This is taking longer than usual. The 3D assets may still be downloading.</span><button type="button" className="button button-secondary" onClick={onSkip}>Enter the table anyway</button></div> : null}
    </div>
  </div>
}
