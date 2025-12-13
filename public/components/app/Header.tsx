import React, { useState } from "react"
import { SignInModal, TenantLogo, NotificationIndicator, UserMenu, ModIndicator, QueueIndicator, Icon } from "@fider/components"
import { useFider } from "@fider/hooks"
import { HStack } from "../layout"
import { Trans } from "@lingui/react/macro"
import { UnreadCountsProvider } from "@fider/contexts/UnreadCountsContext"
import { heroiconsHome as IconHome } from "@fider/icons.generated"

export const Header = () => {
  const fider = useFider()
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const isHomePage = typeof window !== "undefined" && (window.location.pathname === "/" || window.location.pathname === "")

  const showModal = (e: React.MouseEvent) => {
    e.preventDefault()
    setIsSignInModalOpen(true)
  }

  const hideModal = () => setIsSignInModalOpen(false)

  return (
    <div id="c-header" className="bg-elevated">
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <HStack className="p-6 w-full">
        <div className="container">
          <HStack justify="between">
            <HStack spacing={2} className="min-w-0">
              {!isHomePage && (
                <a 
                  href="/" 
                  className="shrink-0 inline-flex items-center justify-center w-8 h-8 rounded-button border border-border bg-surface text-muted hover:bg-tertiary hover:text-foreground hover:border-border-strong transition-all duration-50 md:hidden"
                >
                  <Icon sprite={IconHome} className="w-4 h-4" />
                </a>
              )}
              <a href="/" className="flex items-center gap-2 h-8 min-w-0">
                <span className="shrink-0"><TenantLogo size={100} /></span>
                <h1 className="text-xl font-medium text-foreground truncate max-sm:hidden">{fider.session.tenant.name}</h1>
              </a>
            </HStack>
            {fider.session.isAuthenticated && (
              <UnreadCountsProvider>
                <HStack spacing={4} className="shrink-0">
                  <QueueIndicator />
                  <ModIndicator />
                  <NotificationIndicator />
                  <UserMenu />
                </HStack>
              </UnreadCountsProvider>
            )}
            {!fider.session.isAuthenticated && (
              <a id="c-header-sign-in" href="#" className="uppercase text-sm text-foreground" onClick={showModal}>
                <Trans id="action.signin">Sign in</Trans>
              </a>
            )}
          </HStack>
        </div>
      </HStack>
    </div>
  )
}
