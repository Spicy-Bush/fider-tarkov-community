import React, { useState } from "react"
import { Icon, SignInModal } from "@fider/components"
import { swipeCards as IconSwipeCards } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"
import "./SwipeModeButton.scss"

interface SwipeModeButtonProps {
  onClick: () => void
}

export const SwipeModeButton: React.FC<SwipeModeButtonProps> = ({ onClick }) => {
  const fider = useFider()
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)

  const handleClick = () => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }
    onClick()
  }

  return (
    <>
      <button
        className="c-swipe-mode-button"
        onClick={handleClick}
        aria-label="Open swipe mode for quick voting"
      >
        <Icon sprite={IconSwipeCards} className="h-5 w-5" />
      </button>
      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
    </>
  )
}
