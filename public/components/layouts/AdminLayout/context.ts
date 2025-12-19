import { createContext, useContext } from "react"
import { LayoutVariant } from "@fider/contexts/LayoutContext"

export const USER_ROLES = ["visitor", "collaborator", "moderator", "administrator", "helper"] as const
export type UserRoleName = typeof USER_ROLES[number]

export interface AdminLayoutContextType {
  title?: string
  subtitle?: string
  sidebarItem?: string
  layoutVariant: LayoutVariant
  roles: readonly string[]
}

export const AdminLayoutContext = createContext<AdminLayoutContextType | null>(null)

export const useAdminLayout = (): AdminLayoutContextType => {
  const context = useContext(AdminLayoutContext)
  if (!context) {
    throw new Error("useAdminLayout must be used within an AdminLayout")
  }
  return context
}

