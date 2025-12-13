import { useEffect, useRef, useCallback } from "react"

export interface UseStackNavigationOptions<T> {
  onStateChange: (state: T | null) => void
  urlPath?: string
}

export interface UseStackNavigationResult<T> {
  pushState: (state: T) => void
  replaceState: (state: T) => void
  isNavigating: React.MutableRefObject<boolean>
}

export function useStackNavigation<T extends Record<string, unknown>>(
  options: UseStackNavigationOptions<T>
): UseStackNavigationResult<T> {
  const { onStateChange, urlPath } = options
  const isNavigatingRef = useRef(false)
  const onStateChangeRef = useRef(onStateChange)

  useEffect(() => {
    onStateChangeRef.current = onStateChange
  }, [onStateChange])

  useEffect(() => {
    const handlePopState = (event: PopStateEvent) => {
      isNavigatingRef.current = true
      const state = event.state as T | null
      onStateChangeRef.current(state)
      setTimeout(() => {
        isNavigatingRef.current = false
      }, 0)
    }

    window.addEventListener("popstate", handlePopState)
    return () => window.removeEventListener("popstate", handlePopState)
  }, [])

  const pushState = useCallback(
    (state: T) => {
      if (!isNavigatingRef.current) {
        const path = urlPath ?? window.location.href
        window.history.pushState(state, "", path)
      }
    },
    [urlPath]
  )

  const replaceState = useCallback(
    (state: T) => {
      const path = urlPath ?? window.location.href
      window.history.replaceState(state, "", path)
    },
    [urlPath]
  )

  return {
    pushState,
    replaceState,
    isNavigating: isNavigatingRef,
  }
}
