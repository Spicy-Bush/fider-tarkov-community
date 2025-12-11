import React, { useMemo } from "react"
import { Modal, Button, DisplayError, Select, Form, TextArea, Icon, EditOriginalPostPanel } from "@fider/components"
import { Post, PostStatus, Tag } from "@fider/models"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { heroiconsCheck as IconCheck, heroiconsDuplicate as IconCopy } from "@fider/icons.generated"
import { HStack } from "@fider/components/layout"
import { DuplicateSearchPanel } from "./DuplicateSearchPanel"
import { useResponseModal } from "../hooks"

import "./ResponseModal.scss"

interface ResponseModalProps {
  post: Post
  tags: Tag[]
  attachments: string[]
  showModal: boolean
  onCloseModal: () => void
  hasCopiedContent?: boolean
}

export const ResponseModal: React.FC<ResponseModalProps> = ({
  post,
  tags,
  attachments,
  showModal,
  onCloseModal,
  hasCopiedContent: initialCopiedContent,
}) => {
  const modal = useResponseModal({
    post,
    attachments,
    showModal,
    hasCopiedContent: initialCopiedContent,
  })

  const options = useMemo(() => {
    return PostStatus.All.map((s) => {
      const id = `enum.poststatus.${s.value.toString()}`
      return {
        value: s.value.toString(),
        label: i18n._(id, { message: s.title }),
      }
    })
  }, [])

  const handleStatusChange = (opt?: { value: string }) => {
    if (opt) {
      modal.handleStatusChange(opt.value)
    }
  }

  return (
    <>
      <Modal.Window isOpen={showModal} onClose={onCloseModal} center={false} size="large">
        <Modal.Content>
          <Form error={modal.error} className="c-response-form">
            <Select field="status" label="Status" defaultValue={modal.status} options={options} onChange={handleStatusChange} />
            {modal.status === PostStatus.Duplicate.value ? (
              <>
                {post.description && (
                  <div className="p-3 bg-gray-50 rounded mb-3 border border-gray-200">
                    <HStack justify="between" className="mb-1">
                      <span className="text-xs text-muted uppercase">Copy content from this post</span>
                      <Button variant="tertiary" size="small" onClick={modal.handleCopyContent}>
                        <Icon sprite={modal.copiedContent ? IconCheck : IconCopy} className="h-4" />
                        <span className="text-xs">{modal.copiedContent ? "Copied" : "Copy"}</span>
                      </Button>
                    </HStack>
                    <p className="text-sm text-muted m-0">Copy content to paste into the original post when merging</p>
                  </div>
                )}
                {modal.originalNumber > 0 ? (
                  <div className="p-3 bg-gray-100 rounded mb-2">
                    <span className="font-medium">Original Post: </span>
                    <span>#{modal.originalNumber}</span>
                    <HStack spacing={1} className="ml-2">
                      {(modal.copiedContent || attachments.length > 0) && !modal.showEditOriginal && (
                        <Button variant="secondary" size="small" onClick={() => modal.loadAndShowEditPanel(modal.originalNumber)}>
                          Edit Original
                        </Button>
                      )}
                      <Button variant="tertiary" size="small" onClick={() => modal.setShowDuplicateSearch(true)}>
                        Change
                      </Button>
                    </HStack>
                  </div>
                ) : (
                  <Button variant="secondary" onClick={() => modal.setShowDuplicateSearch(true)}>
                    Select Original Post
                  </Button>
                )}
                <DisplayError fields={["originalNumber"]} error={modal.error} />
                <TextArea
                  field="text"
                  label={i18n._("label.reason", { message: "Reason" })}
                  onChange={modal.setText}
                  value={modal.text}
                  minRows={2}
                  placeholder={i18n._("showpost.duplicateresponse.placeholder", {
                    message: "Explain why this is a duplicate (optional)...",
                  })}
                />
                <span className="text-muted text-sm">
                  <Trans id="showpost.responseform.message.mergedvotes">Votes from this post will be merged into original post.</Trans>
                </span>
              </>
            ) : (
              <TextArea
                field="text"
                label={i18n._("label.reason", { message: "Reason" })}
                onChange={modal.setText}
                value={modal.text}
                minRows={5}
                placeholder={i18n._("showpost.responseform.text.placeholder", {
                  message: "What's going on with this post? Let your users know what are your plans...",
                })}
              />
            )}
          </Form>
        </Modal.Content>

        <Modal.Footer>
          <Button variant="primary" onClick={modal.submit}>
            <Trans id="action.submit">Submit</Trans>
          </Button>
          <Button variant="tertiary" onClick={onCloseModal}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      {modal.showDuplicateSearch && (
        <DuplicateSearchPanel
          excludePostNumber={post.number}
          tags={tags}
          isLoading={modal.isLoadingOriginalPost}
          onSelect={modal.handleDuplicateSelect}
          onCancel={modal.handleDuplicateCancel}
        />
      )}

      {modal.showEditOriginal && modal.editingOriginalPost && (
        <EditOriginalPostPanel
          key={modal.editPanelKey}
          post={modal.editingOriginalPost}
          attachments={modal.editingOriginalAttachments}
          imagesToTransfer={attachments}
          isLoading={modal.isLoadingOriginalPost}
          variant="sidebar"
          onSave={modal.handleSaveOriginal}
          onCancel={modal.handleCancelEditOriginal}
        />
      )}
    </>
  )
}
