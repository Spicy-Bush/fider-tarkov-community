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
import { RequestError } from "@fider/services/http"
import * as notify from "@fider/services/notify"

interface VotesModalProps {
  isOpen: boolean
  post: Post
  revision: number
  onClose?: () => void
}

export const VotesModal: React.FC<VotesModalProps> = (props) => {
  const [isLoading, setIsLoading] = useState(false)
  const [query, setQuery] = useState("")
  const [allVotes, setAllVotes] = useState<Vote[]>([])
  const filteredVotes = allVotes.filter((vote) => vote.user.name.toLowerCase().includes(query.toLowerCase()))

  const fider = useFider()

  useEffect(() => {
    if (!props.isOpen) {
      return
    }

    const request = new AbortController()
    setIsLoading(true)

    const load = async () => {
      try {
        const response = await postActions.listVotes(props.post.number, { signal: request.signal })

        if (!request.signal.aborted && response.ok) {
          setAllVotes(response.data)
        }
      } catch (cause) {
        if (!request.signal.aborted) {
          if (!(cause instanceof RequestError)) throw cause

          notify.error("Could not load the voters. Please retry.")
        }
      } finally {
        if (!request.signal.aborted) {
          setIsLoading(false)
        }
      }
    }

    void load()
    return () => request.abort()
  }, [props.isOpen, props.post.number, props.revision])

  const closeModal = async () => {
    if (props.onClose) {
      props.onClose()
    }
  }

  const clearSearch = () => {
    setQuery("")
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
              onChange={setQuery}
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
