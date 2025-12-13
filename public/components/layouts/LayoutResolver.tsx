import React, { ComponentType } from "react"
import { AdminLayout } from "./AdminLayout"
import { AuthLayout } from "./AuthLayout"
import { PublicLayout } from "./PublicLayout"
import { PageConfig } from "./pageConfigs"

export type { PageConfig }

type LayoutType = "admin" | "auth" | "public" | "standalone"

const getLayoutType = (pageName: string): LayoutType => {
  if (pageName.startsWith("Administration/")) {
    return "admin"
  }
  if (pageName.startsWith("Error/")) {
    return "standalone"
  }
  if (
    pageName.startsWith("SignIn/") ||
    pageName.startsWith("SignUp/") ||
    pageName.startsWith("OAuthEcho/")
  ) {
    return "auth"
  }
  return "public"
}

interface LayoutResolverProps {
  pageName: string
  pageComponent: ComponentType<any>
  pageProps: Record<string, any>
  pageConfig?: PageConfig
}

export const LayoutResolver: React.FC<LayoutResolverProps> = ({
  pageName,
  pageComponent: PageComponent,
  pageProps,
  pageConfig,
}) => {
  const layoutType = getLayoutType(pageName)

  const pageContent = <PageComponent {...pageProps} />

  switch (layoutType) {
    case "admin":
      return (
        <AdminLayout
          title={pageConfig?.title}
          subtitle={pageConfig?.subtitle}
          sidebarItem={pageConfig?.sidebarItem}
          layoutVariant={pageConfig?.layoutVariant}
        >
          {pageContent}
        </AdminLayout>
      )
    case "auth":
      return <AuthLayout>{pageContent}</AuthLayout>
    case "public":
      return <PublicLayout>{pageContent}</PublicLayout>
    case "standalone":
    default:
      return pageContent
  }
}

