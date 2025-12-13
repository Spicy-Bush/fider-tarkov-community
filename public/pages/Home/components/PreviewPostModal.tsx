import React from "react"
import { Modal, Button, Avatar, UserName, Moment, Markdown, ImageGallery } from "@fider/components"
import { ImageUpload } from "@fider/models"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { HStack, VStack } from "@fider/components/layout"

interface PreviewPostModalProps {
  isOpen: boolean
  title: string
  description: string
  attachments: ImageUpload[]
  onClose?: () => void
}

export const PreviewPostModal: React.FC<PreviewPostModalProps> = (props) => {
  const fider = useFider()
  const currentDate = new Date()

  const closeModal = () => {
    if (props.onClose) {
      props.onClose()
    }
  }
  
  const validAttachments = props.attachments.filter(attachment => !attachment.remove)
  const isAuthenticated = fider.session && fider.session.isAuthenticated
    
  return (
    <Modal.Window isOpen={props.isOpen} center={false} onClose={closeModal}>
      <Modal.Header>
        <Trans id="modal.previewpost.title">Post Preview</Trans>
      </Modal.Header>
      
      <Modal.Content>
        <VStack spacing={8}>
          <HStack>
            {isAuthenticated ? ( <Avatar user={fider.session.user} />) : ( <div className="c-avatar" style={{ width: '24px', height: '24px', backgroundColor: 'var(--color-surface-alt)' }}></div> )}
            <VStack spacing={1}> {isAuthenticated ? ( <UserName user={fider.session.user} /> ) : ( <span>Anonymous User</span> )}
              <Moment className="text-muted" locale={fider.currentLocale} date={currentDate} />
            </VStack>
          </HStack>
          
          <h1 className="text-large">{props.title}</h1>
          
          <VStack>
            {props.description ? (
              <Markdown className="description" text={props.description} style="full" />
            ) : (
              <em className="text-muted">
                <Trans id="showpost.message.nodescription">No description provided.</Trans>
              </em>
            )}
            
            {validAttachments.filter(a => a.bkey).length > 0 && (
              <ImageGallery bkeys={validAttachments.filter(a => a.bkey).map(a => a.bkey!)} />
            )}
          </VStack>
        </VStack>
      </Modal.Content>

      <Modal.Footer>
        <Button variant="tertiary" onClick={closeModal}>
          <Trans id="action.close">Close</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}