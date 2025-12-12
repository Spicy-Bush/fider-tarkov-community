import React, { useState } from "react"
import { Icon, SignInModal } from "@fider/components"
import { swipeCards as IconSwipeCards } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"

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
        className="flex items-center justify-center w-10 h-10 rounded-button border border-border bg-surface text-foreground cursor-pointer shrink-0 ml-auto transition-colors hover:bg-tertiary hover:border-border-strong active:scale-95 md:hidden"
        onClick={handleClick}
        aria-label="Open swipe mode for quick voting"
      >
        <Icon sprite={IconSwipeCards} className="h-5 w-5" />
      </button>
      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
    </>
  )
}
