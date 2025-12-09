import "./AdminLayout.scss"

import React, { ReactNode, useEffect } from "react"
import { useLayout, LayoutVariant } from "@fider/contexts/LayoutContext"
import { AdminLayoutContext } from "./context"
import { AdminSidebar } from "./AdminSidebar"
import { AdminHeader } from "./AdminHeader"
import { AdminContent } from "./AdminContent"
import { classSet } from "@fider/services"

interface AdminLayoutProps {
  children: ReactNode
  title?: string
  subtitle?: string
  sidebarItem?: string
  layoutVariant?: LayoutVariant
}

interface AdminLayoutComponent extends React.FC<AdminLayoutProps> {
  Sidebar: typeof AdminSidebar
  Header: typeof AdminHeader
  Content: typeof AdminContent
}

const AdminLayoutRoot: React.FC<AdminLayoutProps> = ({
  children,
  title,
  subtitle,
  sidebarItem,
  layoutVariant = "default",
}) => {
  const { sidebarOpen, setSidebarOpen, setLayoutVariant } = useLayout()

  useEffect(() => {
    setLayoutVariant(layoutVariant)
  }, [layoutVariant, setLayoutVariant])

  const className = classSet({
    "c-admin-layout": true,
  })

  const handleOverlayClick = () => {
    setSidebarOpen(false)
  }

  return (
    <AdminLayoutContext.Provider value={{ title, subtitle, sidebarItem, layoutVariant }}>
      <div
        className={className}
        data-sidebar-open={sidebarOpen}
        data-layout-variant={layoutVariant}
      >
        <div
          className="c-admin-layout__overlay"
          onClick={handleOverlayClick}
          aria-hidden="true"
        />
        <AdminSidebar />
        <div className="c-admin-layout__main">
          <AdminHeader />
          <AdminContent>{children}</AdminContent>
        </div>
      </div>
    </AdminLayoutContext.Provider>
  )
}

export const AdminLayout = AdminLayoutRoot as AdminLayoutComponent
AdminLayout.Sidebar = AdminSidebar
AdminLayout.Header = AdminHeader
AdminLayout.Content = AdminContent

