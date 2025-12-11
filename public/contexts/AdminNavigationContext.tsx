import React, { createContext, useContext, useState, useCallback, useRef, ReactNode, useEffect } from "react"
import { ComponentType } from "react"
import { PageConfig } from "@fider/components/layouts"
import { createPageLoader, PageModule } from "@fider/services"

interface ServerData {
  page: string
  props: Record<string, unknown>
  contextID: string
  tenant: unknown
  user: unknown
  settings: unknown
}

interface NavigationState {
  pageName: string
  pageProps: Record<string, unknown>
  pageConfig?: PageConfig
  Component: ComponentType<unknown> | null
}

interface AdminNavigationContextType {
  currentPage: NavigationState | null
  navigate: (href: string) => Promise<void>
  prefetch: (href: string) => void
}

const defaultContext: AdminNavigationContextType = {
  currentPage: null,
  navigate: async () => {},
  prefetch: () => {},
}

const AdminNavigationContext = createContext<AdminNavigationContextType>(defaultContext)

const pageModules = import.meta.glob<PageModule>("../pages/Administration/**/*.page.tsx")

const pageLoader = createPageLoader(pageModules, { pathPrefix: "../pages/" })

const prefetchCache = new Map<string, Promise<ServerData>>()

const extractServerData = (html: string): ServerData | null => {
  const parser = new DOMParser()
  const doc = parser.parseFromString(html, "text/html")
  const serverDataEl = doc.getElementById("server-data")
  if (!serverDataEl) return null

  try {
    return JSON.parse(serverDataEl.textContent || serverDataEl.innerText)
  } catch {
    return null
  }
}

const fetchPageData = async (href: string): Promise<ServerData> => {
  const cached = prefetchCache.get(href)
  if (cached) return cached

  const promise = fetch(href, {
    headers: { Accept: "text/html" },
    credentials: "same-origin",
  })
    .then((res) => {
      if (!res.ok) throw new Error(`Failed to fetch ${href}`)
      return res.text()
    })
    .then((html) => {
      const data = extractServerData(html)
      if (!data) throw new Error("Failed to extract server data")
      return data
    })

  prefetchCache.set(href, promise)
  return promise
}

interface AdminNavigationProviderProps {
  children: ReactNode
  onPageChange?: (state: NavigationState) => void
}

export const AdminNavigationProvider: React.FC<AdminNavigationProviderProps> = ({ children, onPageChange }) => {
  const [currentPage, setCurrentPage] = useState<NavigationState | null>(null)
  const onPageChangeRef = useRef(onPageChange)
  const abortControllerRef = useRef<AbortController | null>(null)

  useEffect(() => {
    onPageChangeRef.current = onPageChange
  }, [onPageChange])

  const lastPathRef = useRef(window.location.pathname)

  useEffect(() => {
    const handlePopState = async () => {
      const href = window.location.pathname
      if (!href.startsWith("/admin")) return
      
      // Skip if the URL path hasn't changed, only the history state changed.
      // This happens when components like useStackNavigation push state entries
      // for in page navigation (e.g. selecting a post, opening duplicate search).
      // without this check, pressing back would reload the entire page component,
      // destroying all local state and causing the page to remount
      if (href === lastPathRef.current) {
        return
      }
      lastPathRef.current = href

      try {
        const serverData = await fetchPageData(href)
        if (!serverData.page.startsWith("Administration/")) {
          window.location.href = href
          return
        }

        const module = await pageLoader.load(serverData.page)
        const newState: NavigationState = {
          pageName: serverData.page,
          pageProps: serverData.props as Record<string, unknown>,
          pageConfig: module.pageConfig,
          Component: module.default,
        }

        setCurrentPage(newState)
        onPageChangeRef.current?.(newState)
      } catch {
        window.location.href = href
      }
    }

    window.addEventListener("popstate", handlePopState)
    return () => window.removeEventListener("popstate", handlePopState)
  }, [])

  const navigate = useCallback(async (href: string) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    abortControllerRef.current = new AbortController()

    const currentPath = window.location.pathname
    if (currentPath === href) return

    try {
      const serverData = await fetchPageData(href)

      if (!serverData.page.startsWith("Administration/")) {
        window.location.href = href
        return
      }

      const module = await pageLoader.load(serverData.page)

      window.history.pushState({ href }, "", href)

      const newState: NavigationState = {
        pageName: serverData.page,
        pageProps: serverData.props as Record<string, unknown>,
        pageConfig: module.pageConfig,
        Component: module.default,
      }

      setCurrentPage(newState)
      onPageChangeRef.current?.(newState)
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") return
      window.location.href = href
    }
  }, [])

  const prefetch = useCallback((href: string) => {
    if (prefetchCache.has(href)) return
    fetchPageData(href).catch(() => {})
  }, [])

  return (
    <AdminNavigationContext.Provider value={{ currentPage, navigate, prefetch }}>
      {children}
    </AdminNavigationContext.Provider>
  )
}

export const useAdminNavigation = (): AdminNavigationContextType => {
  return useContext(AdminNavigationContext)
}
