// AuthLayout converted to Tailwind

import React, { ReactNode } from "react"
import { TenantLogo } from "@fider/components"
import { useFider } from "@fider/hooks"

interface AuthLayoutProps {
  children: ReactNode
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children }) => {
  const fider = useFider()

  return (
    <div className="min-h-screen flex flex-col bg-tertiary">
      <header className="flex justify-center p-6">
        <a href="/" className="block [&_img]:max-h-[60px] [&_img]:w-auto">
          <TenantLogo size={100} />
        </a>
      </header>
      <main className="flex-1 flex justify-center items-start p-4">
        <div className="w-full max-w-[500px] bg-elevated rounded-modal shadow-md p-8">
          {children}
        </div>
      </main>
      <footer className="p-4 text-center">
        <span className="text-sm text-border-strong">{fider.session.tenant.name}</span>
      </footer>
    </div>
  )
}

