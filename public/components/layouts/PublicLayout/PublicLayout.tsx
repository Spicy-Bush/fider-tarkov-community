import "./PublicLayout.scss"

import React, { ReactNode } from "react"
import { Header } from "@fider/components"
import Footer from "@fider/components/Footer"

interface PublicLayoutProps {
  children: ReactNode
}

export const PublicLayout: React.FC<PublicLayoutProps> = ({ children }) => {
  return (
    <div className="c-public-layout">
      <Header />
      <main className="c-public-layout__content">{children}</main>
      <Footer />
    </div>
  )
}

