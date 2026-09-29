import React from "react"
import { MultiImageUploader } from "@fider/components/common/form/MultiImageUploader"
import { DraftImage } from "@fider/services/draftImages"

interface CommentAttachmentsProps {
  attachments: DraftImage[]
  existing?: string[]
  allowUploads: boolean
  maxUploads: number
  disabled: boolean
  onChange: (attachments: DraftImage[]) => void
}

export function CommentAttachments(props: CommentAttachmentsProps) {
  const images = [
    ...props.attachments,
    ...(props.existing || [])
      .filter(bkey => !props.attachments.some(image => (image.kind === "stored" || image.kind === "removed") && image.bkey === bkey))
      .map((bkey): DraftImage => ({ kind: "stored", bkey })),
  ]

  return (
    <MultiImageUploader
      field="attachment"
      value={images}
      maxUploads={props.maxUploads}
      disabled={props.disabled}
      allowUploads={props.allowUploads}
      onChange={props.onChange}
    />
  )
}
