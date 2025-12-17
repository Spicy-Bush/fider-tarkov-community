import React, { ReactNode } from "react"
import { Header, Footer, SubheaderBar } from "@fider/components"

interface PublicLayoutProps {
  children: ReactNode
}

export const PublicLayout: React.FC<PublicLayoutProps> = ({ children }) => {
  return (
    <div className="min-h-screen flex flex-col">
      <Header />
      <SubheaderBar />
      <main className="flex-1 pb-24">{children}</main>
      <Footer />
    </div>
  )
}
