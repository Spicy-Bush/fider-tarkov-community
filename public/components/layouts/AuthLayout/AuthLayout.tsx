import "./AuthLayout.scss"

import React, { ReactNode } from "react"
import { TenantLogo } from "@fider/components"
import { useFider } from "@fider/hooks"

interface AuthLayoutProps {
  children: ReactNode
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children }) => {
  const fider = useFider()

  return (
    <div className="c-auth-layout">
      <header className="c-auth-layout__header">
        <a href="/" className="c-auth-layout__logo">
          <TenantLogo size={100} />
        </a>
      </header>
      <main className="c-auth-layout__content">
        <div className="c-auth-layout__card">{children}</div>
      </main>
      <footer className="c-auth-layout__footer">
        <span className="c-auth-layout__footer-text">{fider.session.tenant.name}</span>
      </footer>
    </div>
  )
}

