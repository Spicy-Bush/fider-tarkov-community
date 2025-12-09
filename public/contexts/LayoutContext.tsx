import React, { createContext, useContext, useState, useEffect, ReactNode, useCallback } from "react"

export type LayoutVariant = "default" | "fullWidth" | "custom"

interface LayoutState {
  sidebarOpen: boolean
  layoutVariant: LayoutVariant
}

interface LayoutContextType {
  sidebarOpen: boolean
  layoutVariant: LayoutVariant
  toggleSidebar: () => void
  setSidebarOpen: (open: boolean) => void
  setLayoutVariant: (variant: LayoutVariant) => void
}

const STORAGE_KEY = "fider:layout"

const defaultState: LayoutState = {
  sidebarOpen: true,
  layoutVariant: "default",
}

const loadFromStorage = (): LayoutState => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      if (typeof parsed.sidebarCollapsed === "boolean") {
        return {
          sidebarOpen: !parsed.sidebarCollapsed,
          layoutVariant: parsed.layoutVariant || defaultState.layoutVariant,
        }
      }
      return {
        sidebarOpen: typeof parsed.sidebarOpen === "boolean" ? parsed.sidebarOpen : defaultState.sidebarOpen,
        layoutVariant: parsed.layoutVariant || defaultState.layoutVariant,
      }
    }
  } catch {
  }
  return defaultState
}

const saveToStorage = (state: LayoutState): void => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
  }
}

const LayoutContext = createContext<LayoutContextType | null>(null)

interface LayoutProviderProps {
  children: ReactNode
}

export const LayoutProvider: React.FC<LayoutProviderProps> = ({ children }) => {
  const [sidebarOpen, setSidebarOpenState] = useState(defaultState.sidebarOpen)
  const [layoutVariant, setLayoutVariantState] = useState<LayoutVariant>(defaultState.layoutVariant)
  const [isHydrated, setIsHydrated] = useState(false)

  useEffect(() => {
    const stored = loadFromStorage()
    setSidebarOpenState(stored.sidebarOpen)
    setLayoutVariantState(stored.layoutVariant)
    setIsHydrated(true)
  }, [])

  useEffect(() => {
    if (isHydrated) {
      saveToStorage({ sidebarOpen, layoutVariant })
    }
  }, [sidebarOpen, layoutVariant, isHydrated])

  const toggleSidebar = useCallback(() => {
    setSidebarOpenState((prev) => !prev)
  }, [])

  const setSidebarOpen = useCallback((open: boolean) => {
    setSidebarOpenState(open)
  }, [])

  const setLayoutVariant = useCallback((variant: LayoutVariant) => {
    setLayoutVariantState(variant)
  }, [])

  return (
    <LayoutContext.Provider
      value={{
        sidebarOpen,
        layoutVariant,
        toggleSidebar,
        setSidebarOpen,
        setLayoutVariant,
      }}
    >
      {children}
    </LayoutContext.Provider>
  )
}

export const useLayout = (): LayoutContextType => {
  const context = useContext(LayoutContext)
  if (!context) {
    throw new Error("useLayout must be used within a LayoutProvider")
  }
  return context
}

