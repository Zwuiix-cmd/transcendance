const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
const HEARTBEAT_GRACE_MS = 60000
const MAX_BACKOFF_MS = 30000

export type RealtimeStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'offline'

export type RealtimeEvent<T = unknown> = {
  topic: string
  event: string
  payload: T
}

type ServerFrame = {
  type: 'ready' | 'subscribed' | 'unsubscribed' | 'event' | 'error' | 'heartbeat' | 'pong'
  topic?: string
  event?: string
  payload?: unknown
  userId?: string | null
  code?: string
  message?: string
}

type Listener = (event: RealtimeEvent) => void

export function realtimeUrl() {
  if (/^https?:\/\//.test(API_BASE_URL)) return `${API_BASE_URL.replace(/^http/, 'ws')}/ws`
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${protocol}://${window.location.host}${API_BASE_URL}/ws`
}

class RealtimeClient {
  private socket: WebSocket | null = null
  private token: string | null = null
  private status: RealtimeStatus = 'idle'
  private readonly listeners = new Map<string, Set<Listener>>()
  private readonly statusListeners = new Set<(status: RealtimeStatus) => void>()
  private readonly resyncListeners = new Set<() => void>()
  private readonly unauthorizedListeners = new Set<() => void>()
  private readonly pendingAcks = new Set<string>()
  private attempt = 0
  private generation = 0
  private wasOpen = false
  private needsResync = false
  private halted = false
  private reconnectTimer: number | undefined
  private watchdogTimer: number | undefined

  constructor() {
    if (typeof window === 'undefined') return
    window.addEventListener('online', () => { this.attempt = 0; this.connect() })
    window.addEventListener('offline', () => { this.teardown('offline') })
  }

  setToken(token: string | null) {
    if (this.token === token) return
    this.token = token
    this.halted = false
    this.attempt = 0
    this.restart()
  }

  getStatus() {
    return this.status
  }

  subscribe(topic: string, listener: Listener) {
    let set = this.listeners.get(topic)
    const first = !set
    if (!set) {
      set = new Set()
      this.listeners.set(topic, set)
    }
    set.add(listener)
    if (first) this.send({ type: 'subscribe', topic })
    this.connect()
    return () => {
      const current = this.listeners.get(topic)
      if (!current) return
      current.delete(listener)
      if (current.size === 0) {
        this.listeners.delete(topic)
        this.send({ type: 'unsubscribe', topic })
      }
    }
  }

  onStatus(listener: (status: RealtimeStatus) => void) {
    this.statusListeners.add(listener)
    return () => { this.statusListeners.delete(listener) }
  }

  onResync(listener: () => void) {
    this.resyncListeners.add(listener)
    return () => { this.resyncListeners.delete(listener) }
  }

  onUnauthorized(listener: () => void) {
    this.unauthorizedListeners.add(listener)
    return () => { this.unauthorizedListeners.delete(listener) }
  }

  private shouldConnect() {
    return !this.halted && (this.token !== null || this.listeners.size > 0)
  }

  private connect() {
    if (this.socket || !this.shouldConnect() || typeof WebSocket === 'undefined') return
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      this.setStatus('offline')
      return
    }
    window.clearTimeout(this.reconnectTimer)
    this.reconnectTimer = undefined
    const generation = ++this.generation
    this.setStatus(this.wasOpen ? 'reconnecting' : 'connecting')
    let socket: WebSocket
    try {
      socket = new WebSocket(realtimeUrl())
    } catch {
      this.scheduleReconnect()
      return
    }
    this.socket = socket
    socket.onopen = () => {
      if (generation !== this.generation) return
      socket.send(JSON.stringify({ type: 'auth', token: this.token ?? '' }))
      this.armWatchdog()
    }
    socket.onmessage = (message) => {
      if (generation !== this.generation) return
      this.armWatchdog()
      let frame: ServerFrame
      try {
        frame = JSON.parse(String(message.data)) as ServerFrame
      } catch {
        return
      }
      this.handle(frame)
    }
    socket.onclose = (event) => {
      if (generation !== this.generation) return
      this.socket = null
      window.clearTimeout(this.watchdogTimer)
      if (event.code === 4401) {
        this.halted = true
        this.setStatus('idle')
        this.unauthorizedListeners.forEach((listener) => listener())
        return
      }
      this.scheduleReconnect()
    }
    socket.onerror = () => {
      if (generation !== this.generation) return
      socket.close()
    }
  }

  private handle(frame: ServerFrame) {
    switch (frame.type) {
      case 'ready': {
        this.attempt = 0
        this.setStatus('open')
        this.pendingAcks.clear()
        for (const topic of this.listeners.keys()) {
          this.pendingAcks.add(topic)
          this.send({ type: 'subscribe', topic })
        }
        this.needsResync = this.wasOpen
        this.wasOpen = true
        if (this.pendingAcks.size === 0) this.flushResync()
        return
      }
      case 'subscribed':
      case 'error': {
        if (frame.topic && this.pendingAcks.delete(frame.topic) && this.pendingAcks.size === 0) this.flushResync()
        return
      }
      case 'event': {
        if (!frame.topic || !frame.event) return
        const event: RealtimeEvent = { topic: frame.topic, event: frame.event, payload: frame.payload }
        this.listeners.get(frame.topic)?.forEach((listener) => listener(event))
        return
      }
      default:
        return
    }
  }

  private flushResync() {
    if (!this.needsResync) return
    this.needsResync = false
    this.resyncListeners.forEach((listener) => listener())
  }

  private send(message: Record<string, unknown>) {
    if (this.socket?.readyState === WebSocket.OPEN && this.status === 'open') this.socket.send(JSON.stringify(message))
  }

  private armWatchdog() {
    window.clearTimeout(this.watchdogTimer)
    this.watchdogTimer = window.setTimeout(() => {
      this.socket?.close(4000, 'heartbeat timeout')
    }, HEARTBEAT_GRACE_MS)
  }

  private scheduleReconnect() {
    if (!this.shouldConnect()) {
      this.setStatus('idle')
      return
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      this.setStatus('offline')
      return
    }
    this.setStatus(this.wasOpen ? 'reconnecting' : 'connecting')
    const delay = Math.min(MAX_BACKOFF_MS, 1000 * 2 ** this.attempt) + Math.random() * 500
    this.attempt += 1
    window.clearTimeout(this.reconnectTimer)
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = undefined
      this.connect()
    }, delay)
  }

  private teardown(status: RealtimeStatus) {
    this.generation += 1
    window.clearTimeout(this.reconnectTimer)
    window.clearTimeout(this.watchdogTimer)
    this.reconnectTimer = undefined
    const socket = this.socket
    this.socket = null
    socket?.close(1000, 'client restart')
    this.setStatus(status)
  }

  private restart() {
    this.teardown('idle')
    this.wasOpen = false
    this.connect()
  }

  private setStatus(status: RealtimeStatus) {
    if (this.status === status) return
    this.status = status
    this.statusListeners.forEach((listener) => listener(status))
  }
}

export const realtime = new RealtimeClient()
