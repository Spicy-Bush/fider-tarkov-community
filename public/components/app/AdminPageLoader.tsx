import React, { useState, useCallback, ComponentType } from "react"
import { AdminNavigationProvider } from "@fider/contexts/AdminNavigationContext"
import { LayoutResolver, PageConfig } from "@fider/components/layouts"
import { AsyncPageLoader } from "@fider/AsyncPages"

interface AdminPageLoaderProps {
  initialPageName: string
  initialPageProps: Record<string, unknown>
}

interface NavigationState {
  pageName: string
  pageProps: Record<string, unknown>
  pageConfig?: PageConfig
  Component: ComponentType<unknown> | null
}

const AdminPageLoaderInner: React.FC<AdminPageLoaderProps> = ({ initialPageName, initialPageProps }) => {
  const [overridePage, setOverridePage] = useState<NavigationState | null>(null)

  const handlePageChange = useCallback((state: NavigationState) => {
    setOverridePage(state)
  }, [])

  if (overridePage && overridePage.Component) {
    return (
      <AdminNavigationProvider onPageChange={handlePageChange}>
        <LayoutResolver
          pageName={overridePage.pageName}
          pageComponent={overridePage.Component}
          pageProps={overridePage.pageProps}
          pageConfig={overridePage.pageConfig}
        />
      </AdminNavigationProvider>
    )
  }

  return (
    <AdminNavigationProvider onPageChange={handlePageChange}>
      <AsyncPageLoader
        pageName={initialPageName}
        pageProps={initialPageProps}
        renderWithLayout={(Component, pageConfig, props) => (
          <LayoutResolver
            pageName={initialPageName}
            pageComponent={Component}
            pageProps={props}
            pageConfig={pageConfig}
          />
        )}
      />
    </AdminNavigationProvider>
  )
}

export const AdminPageLoader: React.FC<AdminPageLoaderProps> = (props) => {
  return <AdminPageLoaderInner {...props} />
}

