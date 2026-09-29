import { useContext, useSyncExternalStore } from "react"
import { FiderContext } from "@fider/services/fider"

export const useFider = () => {
  const fider = useContext(FiderContext)
  const session = fider.session
  useSyncExternalStore(session.subscribe, session.getSnapshot, session.getSnapshot)

  return fider
}

export const useCurrentUser = () => {
  const { session } = useContext(FiderContext)
  return useSyncExternalStore(session.subscribe, session.getUserSnapshot, session.getUserSnapshot)
}
