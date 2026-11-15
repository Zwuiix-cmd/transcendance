type VitalName = 'LCP' | 'CLS' | 'INP' | 'FCP' | 'TTFB'

type VitalState = { lcp: number; cls: number; inp: number; fcp: number; ttfb: number }

const state: VitalState = { lcp: 0, cls: 0, inp: 0, fcp: 0, ttfb: 0 }

function send(name: VitalName, value: number) {
  if (!Number.isFinite(value) || value < 0 || !navigator.sendBeacon) return
  const body = JSON.stringify({ name, value: Math.round(value * 100) / 100, path: window.location.pathname })
  navigator.sendBeacon('/api/v1/telemetry/vitals', new Blob([body], { type: 'application/json' }))
}

function observe(type: string, callback: (entry: PerformanceEntry) => void) {
  if (typeof PerformanceObserver === 'undefined' || !PerformanceObserver.supportedEntryTypes?.includes(type)) return
  try {
    new PerformanceObserver((list) => list.getEntries().forEach(callback)).observe({ type, buffered: true } as PerformanceObserverInit)
  } catch {
    // Older browsers can still use the rest of the application without metrics.
  }
}

export function startWebVitals() {
  const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
  if (navigation) state.ttfb = navigation.responseStart - navigation.requestStart

  const firstPaint = performance.getEntriesByName('first-contentful-paint')[0]
  if (firstPaint) state.fcp = firstPaint.startTime
  observe('paint', (entry) => {
    if (entry.name === 'first-contentful-paint') state.fcp = entry.startTime
  })
  observe('largest-contentful-paint', (entry) => { state.lcp = entry.startTime })
  observe('layout-shift', (entry) => {
    const shift = entry as PerformanceEntry & { hadRecentInput?: boolean; value?: number }
    if (!shift.hadRecentInput) state.cls += shift.value ?? 0
  })
  observe('event', (entry) => {
    const event = entry as PerformanceEntry & { duration?: number }
    state.inp = Math.max(state.inp, event.duration ?? 0)
  })

  const report = () => {
    send('LCP', state.lcp)
    send('CLS', state.cls)
    send('INP', state.inp)
    send('FCP', state.fcp)
    send('TTFB', state.ttfb)
  }
  window.addEventListener('pagehide', report, { once: true })
  window.setTimeout(report, 10000)
}
