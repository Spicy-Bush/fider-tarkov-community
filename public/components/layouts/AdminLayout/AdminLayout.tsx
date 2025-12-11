import "./AdminLayout.scss"

import React, { ReactNode, useEffect, useState } from "react"
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
  const [touchStart, setTouchStart] = useState<{ x: number; y: number; time: number; inEdge: boolean } | null>(null)

  useEffect(() => {
    setLayoutVariant(layoutVariant)
  }, [layoutVariant, setLayoutVariant])

  useEffect(() => {
    const isMobile = window.matchMedia("(max-width: 640px)").matches
    if (!isMobile) return

    const handleTouchStart = (e: TouchEvent) => {
      const touch = e.touches[0]
      const edgeZone = window.innerWidth * 0.2
      setTouchStart({
        x: touch.clientX,
        y: touch.clientY,
        time: Date.now(),
        inEdge: touch.clientX < edgeZone,
      })
    }

    const handleTouchEnd = (e: TouchEvent) => {
      if (!touchStart) return

      const touch = e.changedTouches[0]
      const deltaX = touch.clientX - touchStart.x
      const deltaY = touch.clientY - touchStart.y
      const elapsed = Date.now() - touchStart.time

      const angle = Math.abs(Math.atan2(deltaY, deltaX) * (180 / Math.PI))
      const isHorizontal = angle < 25 || angle > 155

      if (!isHorizontal || elapsed > 500) {
        setTouchStart(null)
        return
      }

      if (!sidebarOpen && touchStart.inEdge && deltaX > 80) {
        setSidebarOpen(true)
      } else if (sidebarOpen && deltaX < -80) {
        setSidebarOpen(false)
      }

      setTouchStart(null)
    }

    document.addEventListener("touchstart", handleTouchStart, { passive: true })
    document.addEventListener("touchend", handleTouchEnd, { passive: true })

    return () => {
      document.removeEventListener("touchstart", handleTouchStart)
      document.removeEventListener("touchend", handleTouchEnd)
    }
  }, [touchStart, sidebarOpen, setSidebarOpen])

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

