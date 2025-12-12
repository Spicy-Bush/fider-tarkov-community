// DevBanner converted to Tailwind

import React from "react"
import { useFider } from "@fider/hooks"

export const DevBanner = () => {
  const fider = useFider()

  if (fider.isProduction()) {
    return null
  }

  return (
    <div className="fixed top-2 left-2 p-2 text-base text-danger-hover border-2 border-danger-hover bg-danger-light opacity-70">
      DEV
    </div>
  )
}
