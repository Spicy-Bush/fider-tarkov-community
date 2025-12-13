import { useState, useCallback } from "react"

export const useModalState = <T extends string>() => {
  const [activeModal, setActiveModal] = useState<T | null>(null)

  const isOpen = useCallback((name: T) => activeModal === name, [activeModal])
  
  const open = useCallback((name: T) => {
    setActiveModal(name)
  }, [])
  
  const close = useCallback(() => {
    setActiveModal(null)
  }, [])

  return {
    activeModal,
    isOpen,
    open,
    close,
  }
}

