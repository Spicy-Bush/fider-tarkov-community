import React, { useState, useEffect, ComponentType } from "react"
import { Loader } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"
import { createPageLoader, PageModule } from "@fider/services"

const pageModules = import.meta.glob<PageModule>("./pages/**/*.page.tsx")

const pageLoader = createPageLoader(pageModules, { pathPrefix: "./pages/" })

interface PageLoaderResult {
  Component: ComponentType<any> | null
  pageConfig: PageConfig | undefined
  isLoading: boolean
  error: Error | null
}

export const usePageLoader = (pageName: string): PageLoaderResult => {
  const [result, setResult] = useState<PageLoaderResult>(() => {
    const cached = pageLoader.getCached(pageName)
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

    pageLoader.load(pageName)
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

export type { PageModule }
