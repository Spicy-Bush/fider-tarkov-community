import { useContext, useSyncExternalStore } from "react"
import { FiderContext } from "@fider/services"

export const useFider = () => {
  const fider = useContext(FiderContext)
  const session = fider.session
  useSyncExternalStore(session.subscribeUser, session.getUserSnapshot, session.getUserSnapshot)

  return fider
}