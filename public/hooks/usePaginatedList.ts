import { useState, useEffect, useCallback, useRef } from "react"

export interface UsePaginatedListOptions<T> {
  fetchFn: (page: number, perPage: number) => Promise<T[]>
  perPage?: number
  keyExtractor: (item: T) => string | number
}

export interface UsePaginatedListResult<T> {
  items: T[]
  total: number
  page: number
  perPage: number
  isLoading: boolean
  setPage: (page: number) => void
  reload: () => void
  updateItem: (id: string | number, changes: Partial<T>) => void
  removeItem: (id: string | number) => void
  addItem: (item: T, position?: "start" | "end") => void
  setItems: React.Dispatch<React.SetStateAction<T[]>>
}

export function usePaginatedList<T>(
  options: UsePaginatedListOptions<T>
): UsePaginatedListResult<T> {
  const { fetchFn, perPage = 50, keyExtractor } = options
  const [items, setItems] = useState<T[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const fetchIdRef = useRef(0)

  const loadItems = useCallback(async () => {
    const fetchId = ++fetchIdRef.current
    setIsLoading(true)

    try {
      const result = await fetchFn(page, perPage)

      if (fetchId !== fetchIdRef.current) {
        return
      }

      setItems(result || [])
      const resultCount = result?.length || 0
      setTotal(resultCount === perPage ? page * perPage + 1 : page * perPage)
    } catch {
      if (fetchId === fetchIdRef.current) {
        setItems([])
        setTotal(0)
      }
    } finally {
      if (fetchId === fetchIdRef.current) {
        setIsLoading(false)
      }
    }
  }, [fetchFn, page, perPage])

  useEffect(() => {
    loadItems()
  }, [loadItems])

  const reload = useCallback(() => {
    loadItems()
  }, [loadItems])

  const updateItem = useCallback(
    (id: string | number, changes: Partial<T>) => {
      setItems((prev) =>
        prev.map((item) =>
          keyExtractor(item) === id ? { ...item, ...changes } : item
        )
      )
    },
    [keyExtractor]
  )

  const removeItem = useCallback(
    (id: string | number) => {
      setItems((prev) => prev.filter((item) => keyExtractor(item) !== id))
    },
    [keyExtractor]
  )

  const addItem = useCallback((item: T, position: "start" | "end" = "start") => {
    setItems((prev) =>
      position === "start" ? [item, ...prev] : [...prev, item]
    )
  }, [])

  return {
    items,
    total,
    page,
    perPage,
    isLoading,
    setPage,
    reload,
    updateItem,
    removeItem,
    addItem,
    setItems,
  }
}

