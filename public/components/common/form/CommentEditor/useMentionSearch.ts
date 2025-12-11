import { useState, useEffect } from "react"
import { UserNames } from "@fider/models"
import { actions } from "@fider/services"
import { Range } from "slate"

interface UseMentionSearchResult {
  users: UserNames[]
  isSearching: boolean
}

export const useMentionSearch = (search: string, target: Range | undefined): UseMentionSearchResult => {
  const [users, setUsers] = useState<UserNames[]>([])
  const [isSearching, setIsSearching] = useState(false)

  useEffect(() => {
    if (target) {
      setIsSearching(true)
      const loadUsers = async () => {
        const result = await actions.getTaggableUsers(search)
        if (result.ok) {
          setUsers(result.data)
        }
        setIsSearching(false)
      }
      loadUsers()
    }
  }, [search, target])

  return { users, isSearching }
}

