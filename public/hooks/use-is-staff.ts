import { permissions } from "@fider/services"

export const useIsStaff = (): boolean => {
  return permissions.admin.canAccessAdmin()
}

