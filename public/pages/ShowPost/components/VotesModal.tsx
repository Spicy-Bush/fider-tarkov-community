import React, { useEffect, useState } from "react"
import { Post, Vote } from "@fider/models"
import { Modal } from "@fider/components/common/Modal"
import { Button } from "@fider/components/common/Button"
import { Loader } from "@fider/components/common/Loader"
import { Avatar } from "@fider/components/common/Avatar"
import { UserName } from "@fider/components/common/UserName"
import { Moment } from "@fider/components/common/Moment"
import { Input } from "@fider/components/common/form/Input"
import * as postActions from "@fider/services/actions/post"
import { useFider } from "@fider/hooks/use-fider"
import { heroiconsSearch as IconSearch, heroiconsX as IconX } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout/Stack"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

interface VotesModalProps {
  isOpen: boolean
  post: Post
  onClose?: () => void
}

export const VotesModal: React.FC<VotesModalProps> = (props) => {
  const [isLoading, setIsLoading] = useState(false)
  const [query, setQuery] = useState("")
  const [allVotes, setAllVotes] = useState<Vote[]>([])
  const [filteredVotes, setFilteredVotes] = useState<Vote[]>([])

  const fider = useFider()

  useEffect(() => {
    if (props.isOpen) {
      postActions.listVotes(props.post.number).then((response) => {
        if (response.ok) {
          setAllVotes(response.data)
          setFilteredVotes(response.data)
          setIsLoading(false)
        }
      })
    }
  }, [props.isOpen])

  const closeModal = async () => {
    if (props.onClose) {
      props.onClose()
    }
  }

  const clearSearch = () => {
    handleSearchFilterChanged("")
  }

  const handleSearchFilterChanged = (query: string) => {
    const votes = allVotes.filter((x) => x.user.name.toLowerCase().indexOf(query.toLowerCase()) >= 0)
    setQuery(query)
    setFilteredVotes(votes)
  }

  return (
    <Modal.Window isOpen={props.isOpen} center={false} onClose={closeModal}>
      <Modal.Content>
        {isLoading && <Loader />}
        {!isLoading && (
          <>
            <Input
              field="query"
              icon={query ? IconX : IconSearch}
              onIconClick={query ? clearSearch : undefined}
              placeholder={i18n._("modal.showvotes.query.placeholder", { message: "Search for users by name..." })}
              value={query}
              onChange={handleSearchFilterChanged}
            />
            <VStack spacing={2} className="h-max-5xl overflow-auto">
              {filteredVotes.map((x) => (
                <HStack key={x.user.id} justify="between">
                  <HStack>
                    <Avatar user={x.user} />
                    <VStack spacing={0}>
                      <UserName user={x.user} />
                      <span className="text-muted">{x.user.email}</span>
                    </VStack>
                  </HStack>
                  <span className="text-muted">
                    <Moment locale={fider.currentLocale} date={x.createdAt} />
                  </span>
                </HStack>
              ))}
              {filteredVotes.length === 0 && (
                <p className="text-muted">
                  <Trans id="modal.showvotes.message.zeromatches">
                    No users found matching <strong>{query}</strong>.
                  </Trans>
                </p>
              )}
            </VStack>
          </>
        )}
      </Modal.Content>

      <Modal.Footer>
        <Button variant="tertiary" onClick={closeModal}>
          <Trans id="action.close">Close</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}
