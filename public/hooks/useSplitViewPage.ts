import { useState, useCallback, useRef, useEffect } from "react"

interface UseSplitViewPageConfig<T, K> {
  fetchItems: (page: number, perPage: number) => Promise<{ items: T[]; total: number }>
  getItemKey: (item: T) => K
  perPage?: number
  fetchDetailOnSelect?: boolean
}

interface UseSplitViewPageResult<T, K> {
  items: T[]
  setItems: React.Dispatch<React.SetStateAction<T[]>>
  total: number
  setTotal: React.Dispatch<React.SetStateAction<number>>
  page: number
  setPage: React.Dispatch<React.SetStateAction<number>>
  perPage: number
  isLoading: boolean
  selectedItem: T | null
  selectedItemRef: React.MutableRefObject<T | null>
  selectItem: (item: T) => void
  deselectItem: () => void
  refresh: () => Promise<void>
  hasMore: boolean
  goToNextPage: () => void
  goToPrevPage: () => void
  newItemKeys: Set<K>
  addNewItemKey: (key: K) => void
  clearNewItemKeys: () => void
  removeItemKey: (key: K) => void
  itemsRef: React.MutableRefObject<T[]>
}

export const useSplitViewPage = <T, K = number>(
  config: UseSplitViewPageConfig<T, K>
): UseSplitViewPageResult<T, K> => {
  const { fetchItems, getItemKey, perPage = 50 } = config

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedItem, setSelectedItem] = useState<T | null>(null)
  const [newItemKeys, setNewItemKeys] = useState<Set<K>>(new Set())

  const selectedItemRef = useRef<T | null>(null)
  const itemsRef = useRef<T[]>([])

  useEffect(() => {
    selectedItemRef.current = selectedItem
  }, [selectedItem])

  useEffect(() => {
    itemsRef.current = items
  }, [items])

  const loadItems = useCallback(async () => {
    setIsLoading(true)
    try {
      const result = await fetchItems(page, perPage)
      setItems(result.items)
      setTotal(result.total)
    } catch {
      setItems([])
      setTotal(0)
    }
    setIsLoading(false)
  }, [fetchItems, page, perPage])

  useEffect(() => {
    loadItems()
  }, [loadItems])

  const selectItem = useCallback((item: T) => {
    setSelectedItem(item)
    const key = getItemKey(item)
    setNewItemKeys((prev) => {
      const newSet = new Set(prev)
      newSet.delete(key)
      return newSet
    })
  }, [getItemKey])

  const deselectItem = useCallback(() => {
    setSelectedItem(null)
  }, [])

  const refresh = useCallback(async () => {
    setNewItemKeys(new Set())
    await loadItems()
  }, [loadItems])

  const hasMore = items.length === perPage

  const goToNextPage = useCallback(() => {
    if (hasMore) {
      setPage((p) => p + 1)
    }
  }, [hasMore])

  const goToPrevPage = useCallback(() => {
    if (page > 1) {
      setPage((p) => p - 1)
    }
  }, [page])

  const addNewItemKey = useCallback((key: K) => {
    setNewItemKeys((prev) => new Set(prev).add(key))
  }, [])

  const clearNewItemKeys = useCallback(() => {
    setNewItemKeys(new Set())
  }, [])

  const removeItemKey = useCallback((key: K) => {
    setItems((prev) => prev.filter((item) => getItemKey(item) !== key))
    if (selectedItem && getItemKey(selectedItem) === key) {
      setSelectedItem(null)
    }
  }, [getItemKey, selectedItem])

  return {
    items,
    setItems,
    total,
    setTotal,
    page,
    setPage,
    perPage,
    isLoading,
    selectedItem,
    selectedItemRef,
    selectItem,
    deselectItem,
    refresh,
    hasMore,
    goToNextPage,
    goToPrevPage,
    newItemKeys,
    addNewItemKey,
    clearNewItemKeys,
    removeItemKey,
    itemsRef,
  }
}

