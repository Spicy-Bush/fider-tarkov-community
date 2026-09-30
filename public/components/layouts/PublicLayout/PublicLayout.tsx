import React, { ReactNode } from "react"
import { Header } from "@fider/components/app/Header"
import { default as Footer } from "@fider/components/app/Footer"
import { SubheaderBar } from "@fider/components/SubheaderBar"
import { SponsorProvider, SponsorSpot } from "@fider/components/sponsorship/SponsorProvider"
import { useFider } from "@fider/hooks"

interface PublicLayoutProps {
  children: ReactNode
}

export const PublicLayout: React.FC<PublicLayoutProps> = ({ children }) => {
  const home = useFider().session.page === "Home/Home.page"

  return (
    <SponsorProvider>
      <div className="min-h-screen flex flex-col">
        <Header />
        <SubheaderBar />
        {!home && (
          <div className="container w-full">
            <SponsorSpot position="navigation" />
          </div>
        )}
        <main className="flex-1 pb-24 outline-none" tabIndex={-1}>{children}</main>
        <Footer />
      </div>
    </SponsorProvider>
  )
}
