import React from "react"
import { Icon } from "@fider/components"
import { swipeCards as IconSwipeCards } from "@fider/icons.generated"
import "./SwipeModeButton.scss"

interface SwipeModeButtonProps {
  onClick: () => void
}

export const SwipeModeButton: React.FC<SwipeModeButtonProps> = ({ onClick }) => {
  return (
    <button
      className="c-swipe-mode-button"
      onClick={onClick}
      aria-label="Open swipe mode for quick voting"
    >
      <Icon sprite={IconSwipeCards} className="h-5 w-5" />
    </button>
  )
}
