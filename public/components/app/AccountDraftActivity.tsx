import { useEffect } from "react"
import { useFider } from "@fider/hooks/use-fider"
import { accountDrafts } from "@fider/services/browserDrafts"

export const AccountDraftActivity = () => {
  const fider = useFider()
  const user = fider.session.isAuthenticated ? fider.session.user : undefined
  const account = `${fider.session.tenant.id}:${user?.id || "anonymous"}`

  useEffect(() => accountDrafts.trackActivity(account), [account])

  return null
}
