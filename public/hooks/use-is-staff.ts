import { useFider } from "./use-fider"

export const useIsStaff = (): boolean => {
  const fider = useFider()
  
  if (!fider.session.isAuthenticated) {
    return false
  }
  
  const user = fider.session.user
  return user.isCollaborator || user.isAdministrator || user.isModerator
}

