import { useState, useCallback, useRef } from "react"

interface CacheEntry<T> {
  data: T
  timestamp: number
}

export interface UseDetailsCacheOptions {
  ttl?: number
}

export interface UseDetailsCacheResult<T> {
  getDetails: <K extends keyof T>(
    key: string | number,
    field: K,
    fetchFn: () => Promise<T[K]>
  ) => Promise<T[K]>
  getCachedDetails: <K extends keyof T>(key: string | number, field: K) => T[K] | undefined
  invalidate: (key: string | number) => void
  invalidateAll: () => void
  isLoading: boolean
}

export function useDetailsCache<T extends Record<string, unknown>>(
  options: UseDetailsCacheOptions = {}
): UseDetailsCacheResult<T> {
  const { ttl = 30000 } = options
  const cache = useRef<Map<string | number, CacheEntry<Partial<T>>>>(new Map())
  const [isLoading, setIsLoading] = useState(false)
  const pendingRequests = useRef<Map<string, Promise<unknown>>>(new Map())

  const isCacheValid = useCallback(
    (entry: CacheEntry<Partial<T>> | undefined): boolean => {
      if (!entry) return false
      return Date.now() - entry.timestamp < ttl
    },
    [ttl]
  )

  const getDetails = useCallback(
    async <K extends keyof T>(
      key: string | number,
      field: K,
      fetchFn: () => Promise<T[K]>
    ): Promise<T[K]> => {
      const cacheKey = `${key}:${String(field)}`
      const existingRequest = pendingRequests.current.get(cacheKey)
      if (existingRequest) {
        return existingRequest as Promise<T[K]>
      }

      const entry = cache.current.get(key)
      if (isCacheValid(entry) && entry?.data[field] !== undefined) {
        return entry.data[field] as T[K]
      }

      setIsLoading(true)
      const request = fetchFn()
        .then((data) => {
          const existingEntry = cache.current.get(key) || { data: {}, timestamp: 0 }
          cache.current.set(key, {
            data: { ...existingEntry.data, [field]: data },
            timestamp: Date.now(),
          })
          return data
        })
        .finally(() => {
          pendingRequests.current.delete(cacheKey)
          setIsLoading(false)
        })

      pendingRequests.current.set(cacheKey, request)
      return request
    },
    [isCacheValid]
  )

  const getCachedDetails = useCallback(
    <K extends keyof T>(key: string | number, field: K): T[K] | undefined => {
      const entry = cache.current.get(key)
      if (isCacheValid(entry)) {
        return entry?.data[field] as T[K] | undefined
      }
      return undefined
    },
    [isCacheValid]
  )

  const invalidate = useCallback((key: string | number) => {
    cache.current.delete(key)
  }, [])

  const invalidateAll = useCallback(() => {
    cache.current.clear()
  }, [])

  return {
    getDetails,
    getCachedDetails,
    invalidate,
    invalidateAll,
    isLoading,
  }
}

