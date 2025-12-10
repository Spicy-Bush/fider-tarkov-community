import React, { useState, useEffect, ComponentType } from "react"
import { Loader } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"

export type PageModule = {
  default: ComponentType<any>
  pageConfig?: PageConfig
}

const pageModules = import.meta.glob<PageModule>("./pages/**/*.page.tsx")

const MAX_RETRIES = 6
const INTERVAL = 1000

const retry = <T,>(fn: () => Promise<T>, retriesLeft = MAX_RETRIES, waitMs = INTERVAL): Promise<T> => {
  return new Promise((resolve, reject) => {
    fn()
      .then(resolve)
      .catch((err) => {
        setTimeout(() => {
          if (retriesLeft === 1) {
            reject(new Error(`${err} after ${MAX_RETRIES} retries`))
            return
          }
          retry(fn, retriesLeft - 1, INTERVAL + INTERVAL).then(resolve, reject)
        }, waitMs)
      })
  })
}

const moduleCache = new Map<string, PageModule>()
const loadingPromises = new Map<string, Promise<PageModule>>()

const loadPageModule = (pageName: string): Promise<PageModule> => {
  const cached = moduleCache.get(pageName)
  if (cached) {
    return Promise.resolve(cached)
  }

  const existingPromise = loadingPromises.get(pageName)
  if (existingPromise) {
    return existingPromise
  }

  const modulePath = `./pages/${pageName}.tsx`
  const loader = pageModules[modulePath]
  if (!loader) {
    return Promise.reject(new Error(`Page not found: ${pageName}`))
  }

  const loadPromise = retry(() => loader()).then((module: PageModule) => {
    moduleCache.set(pageName, module)
    loadingPromises.delete(pageName)
    return module
  })

  loadingPromises.set(pageName, loadPromise)
  return loadPromise
}

interface PageLoaderResult {
  Component: ComponentType<any> | null
  pageConfig: PageConfig | undefined
  isLoading: boolean
  error: Error | null
}

// hook to to load a page module and extract both component and config
export const usePageLoader = (pageName: string): PageLoaderResult => {
  const [result, setResult] = useState<PageLoaderResult>(() => {
    const cached = moduleCache.get(pageName)
    if (cached) {
      return {
        Component: cached.default,
        pageConfig: cached.pageConfig,
        isLoading: false,
        error: null,
      }
    }
    return {
      Component: null,
      pageConfig: undefined,
      isLoading: true,
      error: null,
    }
  })

  useEffect(() => {
    if (result.Component && !result.isLoading) {
      return
    }

    loadPageModule(pageName)
      .then((module) => {
        setResult({
          Component: module.default,
          pageConfig: module.pageConfig,
          isLoading: false,
          error: null,
        })
      })
      .catch((error) => {
        setResult({
          Component: null,
          pageConfig: undefined,
          isLoading: false,
          error,
        })
      })
  }, [pageName])

  return result
}

// component that loads a page and renders it with the layout

interface AsyncPageLoaderProps {
  pageName: string
  pageProps: Record<string, any>
  renderWithLayout: (
    Component: ComponentType<any>,
    pageConfig: PageConfig | undefined,
    pageProps: Record<string, any>
  ) => React.ReactNode
}

export const AsyncPageLoader: React.FC<AsyncPageLoaderProps> = ({
  pageName,
  pageProps,
  renderWithLayout,
}) => {
  const { Component, pageConfig, isLoading, error } = usePageLoader(pageName)

  if (isLoading) {
    return (
      <div className="page">
        <Loader />
      </div>
    )
  }

  if (error || !Component) {
    return (
      <div className="page">
        <div className="container">
          <p>Failed to load page: {error?.message || "Unknown error"}</p>
        </div>
      </div>
    )
  }

  return <>{renderWithLayout(Component, pageConfig, pageProps)}</>
}
