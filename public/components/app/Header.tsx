import React, { useState } from "react"
import { SignInModal, TenantLogo, NotificationIndicator, UserMenu, ModIndicator } from "@fider/components"
import { useFider } from "@fider/hooks"
import { HStack } from "../layout"
import { Trans } from "@lingui/react/macro"
import { UnreadCountsProvider } from "@fider/contexts/UnreadCountsContext"

export const Header = () => {
  const fider = useFider()
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)

  const showModal = (e: React.MouseEvent) => {
    e.preventDefault()
    setIsSignInModalOpen(true)
  }

  const hideModal = () => setIsSignInModalOpen(false)

  return (
    <div id="c-header" className="bg-white">
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <HStack className="c-menu p-4 w-full">
        <div className="container">
          <HStack justify="between">
            <a href="/" className="flex flex-x flex-items-center flex--spacing-2 h-8">
              <TenantLogo size={100} />
              <h1 className="text-header">{fider.session.tenant.name}</h1>
            </a>
            {fider.session.isAuthenticated && (
              <UnreadCountsProvider>
                <HStack spacing={2}>
                  <ModIndicator />
                  <NotificationIndicator />
                  <UserMenu />
                </HStack>
              </UnreadCountsProvider>
            )}
            {!fider.session.isAuthenticated && (
              <a id="c-header-sign-in" href="#" className="uppercase text-sm" onClick={showModal}>
                <Trans id="action.signin">Sign in</Trans>
              </a>
            )}
          </HStack>
        </div>
      </HStack>
    </div>
  )
}

