import React, { ReactNode } from "react"
import { classSet } from "@fider/services"
import { useLayout } from "@fider/contexts/LayoutContext"

interface AdminContentProps {
  children: ReactNode
  noPadding?: boolean
}

export const AdminContent: React.FC<AdminContentProps> = ({ children, noPadding }) => {
  const { layoutVariant } = useLayout()
  const isFullWidth = layoutVariant === "fullWidth"

  return (
    <main className={classSet({
      "flex-1 w-full": true,
      "max-w-[900px] mx-auto": !isFullWidth,
      "p-6 max-md:p-4 max-md:pb-8": !noPadding,
      "p-0": noPadding,
    })}>
      {children}
    </main>
  )
}
