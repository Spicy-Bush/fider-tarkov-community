import { useEffect } from "react"

let locks = 0
let previousOverflow = ""

export function useBodyScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) {
      return
    }

    if (locks === 0) {
      previousOverflow = document.documentElement.style.overflow
      document.documentElement.style.overflow = "hidden"
    }

    locks++

    return () => {
      locks--

      if (locks === 0) {
        document.documentElement.style.overflow = previousOverflow
      }
    }
  }, [active])
}
