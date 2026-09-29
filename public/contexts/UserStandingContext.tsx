import React, { createContext, useContext, useState, useCallback, useEffect, useRef, ReactNode } from "react"
import * as userActions from "@fider/services/actions/user"
import { useFider } from "@fider/hooks/use-fider"
import { UserProfileStanding } from "@fider/models"
import { RequestError } from "@fider/services/http"

interface UserStandingContextType extends UserProfileStanding {
  isLoading: boolean
  isMuted: boolean
  muteReason: string
  error: string | null
  refetch: () => Promise<void>
}

const emptyStanding = { warnings: [], mutes: [] }
const UserStandingContext = createContext<UserStandingContextType>({
  ...emptyStanding,
  isLoading: false,
  isMuted: false,
  muteReason: "",
  error: null,
  refetch: async () => {},
})

export const UserStandingProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const { session } = useFider()
  const contextID = session.contextID
  const user = session.getUserSnapshot()
  const request = useRef<AbortController>()
  const [loadingContext, setLoadingContext] = useState<string>()
  const [failure, setFailure] = useState<{ contextID: string; message: string }>()
  const [details, setDetails] = useState<{ contextID: string; data: UserProfileStanding }>()

  useEffect(() => () => request.current?.abort(), [contextID, user?.id])

  const standing = details && details.contextID === contextID ? details.data : emptyStanding
  const activeMute = standing.mutes.find((mute) => mute.id === user?.latestMuteId)

  const refetch = useCallback(async () => {
    if (!user || session.contextID !== contextID) {
      return
    }

    request.current?.abort()
    const current = new AbortController()
    request.current = current
    setLoadingContext(contextID)
    setFailure(undefined)

    try {
      const result = await userActions.getUserProfileStanding(user.id, current.signal)
      if (current.signal.aborted || session.contextID !== contextID || session.getUserSnapshot()?.id !== user.id) {
        return
      }

      if (result.ok) {
        setDetails({ contextID, data: result.data })
        session.updateUserStanding(result.data)
      } else {
        setFailure({ contextID, message: "Could not load account standing." })
      }
    } catch (error) {
      if (!current.signal.aborted) {
        if (!(error instanceof RequestError)) {
          throw error
        }

        setFailure({ contextID, message: "Could not load account standing." })
      }
    } finally {
      if (request.current === current) {
        request.current = undefined
        setLoadingContext(undefined)
      }
    }
  }, [session, contextID, user?.id])

  return (
    <UserStandingContext.Provider value={{
      ...standing,
      isLoading: loadingContext !== undefined && loadingContext === contextID,
      isMuted: !!user?.isMuted,
      muteReason: user?.isMuted ? activeMute?.reason || "" : "",
      error: failure && failure.contextID === contextID ? failure.message : null,
      refetch,
    }}>
      {children}
    </UserStandingContext.Provider>
  )
}

export const useUserStanding = () => useContext(UserStandingContext)
