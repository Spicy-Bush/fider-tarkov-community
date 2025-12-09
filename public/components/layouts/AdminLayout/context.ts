import { createContext, useContext } from "react"
import { LayoutVariant } from "@fider/contexts/LayoutContext"

export interface AdminLayoutContextType {
  title?: string
  subtitle?: string
  sidebarItem?: string
  layoutVariant: LayoutVariant
}

export const AdminLayoutContext = createContext<AdminLayoutContextType | null>(null)

export const useAdminLayout = (): AdminLayoutContextType => {
  const context = useContext(AdminLayoutContext)
  if (!context) {
    throw new Error("useAdminLayout must be used within an AdminLayout")
  }
  return context
}

