import { useEffect, useRef, useState } from 'react'
import { realtime, type RealtimeEvent, type RealtimeStatus } from './client'

type Handler<T> = (event: RealtimeEvent<T>) => void

export function useRealtimeTopic<T = unknown>(topic: string | null | undefined, handler: Handler<T>) {
  const handlerRef = useRef(handler)
  handlerRef.current = handler
  useEffect(() => {
    if (!topic) return undefined
    return realtime.subscribe(topic, (event) => handlerRef.current(event as RealtimeEvent<T>))
  }, [topic])
}

export function useRealtimeResync(callback: () => void) {
  const callbackRef = useRef(callback)
  callbackRef.current = callback
  useEffect(() => realtime.onResync(() => callbackRef.current()), [])
}

export function useRealtimeStatus() {
  const [status, setStatus] = useState<RealtimeStatus>(realtime.getStatus())
  useEffect(() => realtime.onStatus(setStatus), [])
  return status
}
