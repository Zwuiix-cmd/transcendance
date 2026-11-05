import { useState, type MouseEvent } from 'react'
import type { ActivityMetrics, EloChartGeometry, EloHistoryPoint, PlayerActivityChartGeometry, UserActivityPoint, UserAnalytics, UserGamePoint } from '../types'
import { Card, EmptyState } from './ui'
import { formatDate, formatShortDate } from '../utils/format'
const numberFormatter = new Intl.NumberFormat(undefined)

function formatNumber(value: number) {
  return numberFormatter.format(Math.round(value))
}

function signed(value: number, digits = 0) {
  const rounded = digits === 0 ? Math.round(value) : Number(value.toFixed(digits))
  return (rounded > 0 ? '+' : '') + rounded
}

function reasonLabel(reason: string) {
  return ({
    account_created: 'Account baseline',
    game_result: 'Ranked result',
    admin_adjustment: 'Admin adjustment',
    season_reset: 'Season reset',
  } as Record<string, string>)[reason] ?? reason.replaceAll('_', ' ')
}

function Delta({ value }: { value: number | null | undefined }) {
  if (value === null || value === undefined) return <span className="muted">—</span>
  return <span className={value > 0 ? 'analytics-positive' : value < 0 ? 'analytics-negative' : 'muted'}>{signed(value)}</span>
}

function Metric({ label, value, detail, tone = 'default' }: { label: string; value: string | number; detail?: string; tone?: 'default' | 'positive' | 'negative' }) {
  return <div className={'analytics-metric analytics-metric-' + tone}><span>{label}</span><strong>{typeof value === 'number' ? formatNumber(value) : value}</strong>{detail ? <small>{detail}</small> : null}</div>
}

function EloChart({ points, geometry }: { points: EloHistoryPoint[]; geometry: EloChartGeometry | null }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)

  const chart = {
    points: geometry?.points.map((position, index) => ({
      ...points[index]!,
      x: 32 + position[0] * 696,
      y: 222 - position[1] * 184,
    })) ?? [],
    min: geometry?.min ?? 0,
    max: geometry?.max ?? 0,
  }
  const line = chart.points.map((point) => point.x + ',' + point.y).join(' ')
  const active = hoverIndex === null ? null : chart.points[hoverIndex]
  const handleMove = (event: MouseEvent<SVGRectElement>) => {
    if (chart.points.length === 0) return
    const bounds = event.currentTarget.getBoundingClientRect()
    const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width))
    setHoverIndex(Math.min(chart.points.length - 1, Math.round(ratio * (chart.points.length - 1))))
  }

  return <div className="analytics-chart-wrap">
    {active ? <div className="analytics-tooltip" style={{ left: ((active.x / 760) * 100).toFixed(2) + '%' }}><strong>{formatDate(active.createdAt)}</strong><span>{formatNumber(active.newElo)} ELO · <Delta value={active.delta} /></span><small>{reasonLabel(active.reason)}</small></div> : null}
    <svg className="analytics-chart" viewBox="0 0 760 260" role="img" aria-label="ELO history chart">
      <line x1="32" y1="38" x2="728" y2="38" className="analytics-grid-line" />
      <line x1="32" y1="130" x2="728" y2="130" className="analytics-grid-line" />
      <line x1="32" y1="222" x2="728" y2="222" className="analytics-grid-line" />
      {chart.points.length > 0 ? <><text x="32" y="24" className="analytics-axis-label">{formatNumber(chart.max)}</text><text x="32" y="248" className="analytics-axis-label">{formatNumber(chart.min)}</text><polyline points={line} className="analytics-elo-line" fill="none" /></> : <text x="380" y="132" textAnchor="middle" className="analytics-empty-chart">No ELO events recorded yet</text>}
      <rect x="0" y="0" width="760" height="260" className="analytics-chart-hitbox" onMouseMove={handleMove} onMouseLeave={() => setHoverIndex(null)} aria-hidden="true" />
      {chart.points.map((point, index) => <circle key={'hit-' + point.id} cx={point.x} cy={point.y} r={index === hoverIndex ? 6 : 4} className="analytics-elo-point analytics-elo-point-overlay" tabIndex={0} role="button" aria-label={formatDate(point.createdAt) + ': ' + point.newElo + ' ELO'} onMouseEnter={() => setHoverIndex(index)} onFocus={() => setHoverIndex(index)} onBlur={() => setHoverIndex(null)} />)}
    </svg>
  </div>
}

function ActivityChart({ points, metrics, geometry }: { points: UserActivityPoint[]; metrics: ActivityMetrics; geometry: PlayerActivityChartGeometry | null }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)
  const visiblePoints = points.slice(-30)

  const active = hoverIndex === null ? null : visiblePoints[hoverIndex]
  const handleMove = (event: MouseEvent<SVGRectElement>) => {
    if (visiblePoints.length === 0) return
    const bounds = event.currentTarget.getBoundingClientRect()
    const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width))
    setHoverIndex(Math.min(visiblePoints.length - 1, Math.round(ratio * (visiblePoints.length - 1))))
  }

  return <div className="analytics-chart-wrap">
    {active ? <div className="analytics-tooltip analytics-tooltip-activity" style={{ left: (((hoverIndex ?? 0) / Math.max(1, visiblePoints.length - 1)) * 92 + 4).toFixed(2) + '%' }}><strong>{formatDate(active.date)}</strong><span>{active.games} games · {active.wins} wins</span><small>{formatNumber(active.score)} score · {signed(active.eloDelta)} ELO Δ</small></div> : null}
    <svg className="analytics-chart analytics-activity-chart" viewBox="0 0 760 260" role="img" aria-label="Daily activity chart">
      <line x1="32" y1="38" x2="728" y2="38" className="analytics-grid-line" />
      <line x1="32" y1="130" x2="728" y2="130" className="analytics-grid-line" />
      <line x1="32" y1="222" x2="728" y2="222" className="analytics-grid-line" />
      {visiblePoints.length > 0 ? visiblePoints.map((point, index) => {
        const chartPoint = geometry?.points[index] ?? [0.5, 0, 0]
        const x = 42 + chartPoint[0] * 676
        const totalHeight = chartPoint[1] * 174
        const winHeight = chartPoint[2] * 174
        return <g key={point.date} onMouseEnter={() => setHoverIndex(index)}><rect x={x - 8} y={222 - totalHeight} width="16" height={totalHeight} className="analytics-bar-total" /><rect x={x - 8} y={222 - winHeight} width="16" height={winHeight} className="analytics-bar-wins" /><text x={x} y="244" textAnchor="middle" className="analytics-axis-label">{visiblePoints.length <= 12 || index % 5 === 0 ? formatShortDate(point.date) : ''}</text></g>
      }) : <text x="380" y="132" textAnchor="middle" className="analytics-empty-chart">No finished games recorded yet</text>}
      <rect x="0" y="0" width="760" height="260" className="analytics-chart-hitbox" onMouseMove={handleMove} onMouseLeave={() => setHoverIndex(null)} aria-hidden="true" />
    </svg>
    <div className="analytics-chart-legend"><span><i className="legend-swatch legend-total" />Games</span><span><i className="legend-swatch legend-wins" />Wins</span><small>{String(metrics.activeDays) + ' active days · ' + metrics.averageGamesPerActiveDay.toFixed(1) + ' games/day'}</small></div>
  </div>
}

function GameHistoryTable({ games }: { games: UserGamePoint[] }) {
  if (games.length === 0) return <EmptyState title="No finished games yet" description="Completed matches will appear here with their placement, score and rating impact." />
  return <div className="table-wrap analytics-table-wrap"><table className="analytics-table"><thead><tr><th>Played</th><th>Mode</th><th>Result</th><th>ELO Δ</th><th>Score</th></tr></thead><tbody>{games.map((game) => <tr key={game.gameId}><td>{formatDate(game.playedAt)}</td><td><span className={'analytics-mode analytics-mode-' + game.mode}>{game.mode}</span></td><td>{game.position === 1 ? <span className="analytics-positive">Won</span> : game.position ? '#' + game.position : '—'}</td><td><Delta value={game.eloDelta} /></td><td>{formatNumber(game.score)}</td></tr>)}</tbody></table></div>
}

function EloLedger({ points }: { points: EloHistoryPoint[] }) {
  if (points.length === 0) return <EmptyState title="No ELO history" description="The rating ledger will be populated as the player enters ranked matches." />
  return <div className="table-wrap analytics-table-wrap analytics-ledger-wrap"><table className="analytics-table"><thead><tr><th>Date</th><th>Reason</th><th>Before</th><th>After</th><th>Change</th><th>Reference</th></tr></thead><tbody>{[...points].reverse().map((point) => <tr key={point.id}><td>{formatDate(point.createdAt)}</td><td>{reasonLabel(point.reason)}</td><td>{formatNumber(point.previousElo)}</td><td><strong>{formatNumber(point.newElo)}</strong></td><td><Delta value={point.delta} /></td><td>{point.gameId ? <code>{point.gameId.slice(0, 8)}</code> : <span className="muted">—</span>}</td></tr>)}</tbody></table></div>
}

export function PlayerAnalytics({ analytics }: { analytics: UserAnalytics }) {
  const { summary } = analytics
  const activityMetrics = analytics.activityMetrics
  const positiveTone = summary.netEloDelta > 0 ? 'positive' : summary.netEloDelta < 0 ? 'negative' : 'default'
  const winRate = Math.round(summary.winRate * 100) + '%'
  const activeDays = activityMetrics.activeDays
  return <section className="player-analytics" aria-label="Player analytics">
    <div className="analytics-section-heading"><div><span className="eyebrow">Performance lab</span><h2>Player analytics</h2></div><span className="muted">{summary.eloEvents} rating events · {summary.totalPlayers ? 'rank #' + summary.rank + ' of ' + summary.totalPlayers : 'unranked'}</span></div>
    <div className="analytics-summary-grid">
      <Metric label="Current ELO" value={summary.currentElo} detail={'Peak ' + formatNumber(summary.peakElo)} />
      <Metric label="Average ELO" value={summary.averageElo} detail={'Low ' + formatNumber(summary.lowestElo)} />
      <Metric label="Net ELO Δ" value={signed(summary.netEloDelta)} detail={summary.positiveEloDeltas + ' up · ' + summary.negativeEloDeltas + ' down'} tone={positiveTone} />
      <Metric label="Volatility" value={summary.volatility.toFixed(1)} detail="Average event spread" />
      <Metric label="Percentile" value={Math.round(summary.percentile) + '%'} detail={summary.rank ? 'Position #' + summary.rank : 'No ranking'} />
      <Metric label="Win rate" value={winRate} detail={formatNumber(summary.gamesWon) + ' wins / ' + formatNumber(summary.gamesPlayed) + ' games'} />
      <Metric label="Avg. ELO Δ" value={signed(summary.averageEloDelta, 1)} detail="Per rating event" />
      <Metric label="Current streak" value={summary.currentStreak} detail={'Best ' + summary.maxStreak} />
      <Metric label="Active days" value={activeDays} detail="Activity summary" />
    </div>
    <div className="analytics-chart-grid">
      <Card className="analytics-card analytics-chart-card"><div className="analytics-card-heading"><div><span className="eyebrow">Rating history</span><h3>ELO over time</h3></div><span className="muted">{analytics.eloHistory.length} events</span></div><EloChart points={analytics.eloHistory} geometry={analytics.eloChart} /></Card>
      <Card className="analytics-card analytics-chart-card"><div className="analytics-card-heading"><div><span className="eyebrow">Match activity</span><h3>Games and wins</h3></div><span className="muted">{summary.gamesPlayed} total games</span></div><ActivityChart points={analytics.activity} metrics={activityMetrics} geometry={analytics.activityChart} /></Card>
    </div>
    <div className="analytics-breakdown-grid">
      <Card className="analytics-card"><div className="analytics-card-heading"><div><span className="eyebrow">Career output</span><h3>Uno record</h3></div></div><div className="analytics-breakdown-list"><div><span>UNO calls</span><strong>{formatNumber(summary.unosCalled)}</strong></div><div><span>UNO penalties</span><strong>{formatNumber(summary.unoPenalties)}</strong></div><div><span>+4 played</span><strong>{formatNumber(summary.drawFoursPlayed)}</strong></div><div><span>Cards drawn</span><strong>{formatNumber(summary.cardsDrawn)}</strong></div></div></Card>
      <Card className="analytics-card"><div className="analytics-card-heading"><div><span className="eyebrow">Totals</span><h3>Activity volume</h3></div></div><div className="analytics-breakdown-list"><div><span>Game score</span><strong>{formatNumber(summary.totalScore)}</strong></div><div><span>ELO Δ from games</span><strong>{signed(summary.totalEloDelta)}</strong></div><div><span>Positive ELO Δ</span><strong className="analytics-positive">{formatNumber(summary.positiveEloDeltas)}</strong></div><div><span>Negative ELO Δ</span><strong className="analytics-negative">{formatNumber(summary.negativeEloDeltas)}</strong></div></div></Card>
    </div>
    <Card className="analytics-card analytics-games-card"><div className="analytics-card-heading"><div><span className="eyebrow">Match history</span><h3>Recent finished games</h3></div><span className="muted">Last {Math.min(50, analytics.recentGames.length)}</span></div><GameHistoryTable games={analytics.recentGames} /></Card>
    <Card className="analytics-card analytics-ledger-card"><div className="analytics-card-heading"><div><span className="eyebrow">Audit trail</span><h3>Every ELO movement</h3></div><span className="muted">Immutable ledger</span></div><EloLedger points={analytics.eloHistory} /></Card>
  </section>
}
