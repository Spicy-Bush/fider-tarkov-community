import { useCallback, useLayoutEffect, useRef } from "react"

interface Reader {
  key: string
  capture: () => unknown
}

export type ReadingPosition = Record<string, unknown>

interface Restoration {
  saved: ReadingPosition
  pending: Promise<void>[]
}

const readers = new Set<Reader>()
let restoration: Restoration | undefined

export function captureReadingPosition(): ReadingPosition | undefined {
  if (readers.size === 0) {
    return undefined
  }

  const saved: ReadingPosition = {}

  for (const reader of readers) {
    saved[reader.key] = reader.capture()
  }

  return saved
}

export function prepareReadingPosition(saved: ReadingPosition | undefined): void {
  restoration = saved ? { saved, pending: [] } : undefined
}

export async function finishReadingPosition(signal: AbortSignal): Promise<void> {
  const current = restoration

  if (current && !signal.aborted) {
    let cancel: () => void = () => {}
    const cancelled = new Promise<void>((resolve) => {
      cancel = resolve
      signal.addEventListener("abort", cancel, { once: true })
    })

    await Promise.race([Promise.all(current.pending), cancelled])
    signal.removeEventListener("abort", cancel)
  }

  if (restoration === current) {
    restoration = undefined
  }
}

export function savedReadingPosition<T>(key: string): T | undefined {
  return restoration?.saved[key] as T | undefined
}

export function useReadingPosition<T>(key: string, capture: () => T): {
  saved?: T
  cancelled: Readonly<{ current: boolean }>
  restored: () => void
} {
  const current = useRef(restoration)
  const captureRef = useRef(capture)
  captureRef.current = capture
  const restored = useRef<() => void>(() => {})
  const cancelled = useRef(false)
  const interactions = useRef<AbortController>()
  const saved = current.current?.saved[key] as T | undefined
  const complete = useCallback(() => {
    interactions.current?.abort()
    restored.current()
  }, [])

  useLayoutEffect(() => {
    const reader = { key, capture: () => captureRef.current() }
    readers.add(reader)

    if (saved !== undefined) {
      current.current!.pending.push(new Promise<void>((resolve) => {
        restored.current = resolve
      }))

      const input = new AbortController()
      interactions.current = input
      const cancel = () => {
        cancelled.current = true
        complete()
      }
      const keyDown = (event: KeyboardEvent) => {
        if (["ArrowUp", "ArrowDown", "PageUp", "PageDown", "Home", "End", " "].includes(event.key)) {
          cancel()
        }
      }

      window.addEventListener("wheel", cancel, { signal: input.signal, passive: true })
      window.addEventListener("touchstart", cancel, { signal: input.signal, passive: true })
      window.addEventListener("pointerdown", cancel, { signal: input.signal })
      window.addEventListener("keydown", keyDown, { signal: input.signal })
    }

    return () => {
      readers.delete(reader)
      complete()
    }
  }, [key])

  return { saved, cancelled, restored: complete }
}
