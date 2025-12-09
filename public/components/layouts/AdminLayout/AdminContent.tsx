import React, { ReactNode } from "react"

interface AdminContentProps {
  children: ReactNode
  noPadding?: boolean
}

export const AdminContent: React.FC<AdminContentProps> = ({ children, noPadding }) => {
  const className = noPadding ? "c-admin-content c-admin-content--no-padding" : "c-admin-content"

  return <main className={className}>{children}</main>
}

