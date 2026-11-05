import { useEffect, useRef } from 'react'

type PollCallback = () => void | Promise<void>

/** Polls only while the page is visible and refreshes when it becomes visible again. */
export function usePagePolling(callback: PollCallback, intervalMs: number, enabled = true) {
  const callbackRef = useRef(callback)
  callbackRef.current = callback

  useEffect(() => {
    if (!enabled) return undefined

    let timer: number | undefined
    const stop = () => {
      if (timer !== undefined) {
        window.clearInterval(timer)
        timer = undefined
      }
    }
    const poll = () => {
      if (document.visibilityState === 'visible') void callbackRef.current()
    }
    const start = () => {
      stop()
      if (document.visibilityState === 'visible') {
        timer = window.setInterval(poll, intervalMs)
      }
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        void callbackRef.current()
        start()
      } else {
        stop()
      }
    }

    start()
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      stop()
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [enabled, intervalMs])
}
