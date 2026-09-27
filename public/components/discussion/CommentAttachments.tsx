import React from "react"
import { Button, ImagePicker } from "@fider/components"
import { uploadedImageURL } from "@fider/services"
import { CommentAttachment, commentDrafts } from "@fider/services/commentDrafts"

interface CommentAttachmentsProps {
  attachments: CommentAttachment[]
  existing?: string[]
  allowUploads: boolean
  maxUploads: number
  disabled: boolean
  onChange: (attachments: CommentAttachment[]) => void
}

export function CommentAttachments(props: CommentAttachmentsProps) {
  const images = [
    ...props.attachments,
    ...(props.existing || [])
      .filter((bkey) => !props.attachments.some((image) => !("fileId" in image) && image.bkey === bkey))
      .map((bkey): CommentAttachment => ({ bkey, remove: false })),
  ].filter((image) => "fileId" in image || !image.remove)

  const remove = (image: CommentAttachment) => {
    const attachments = props.attachments.filter((candidate) => candidate !== image)

    if (!("fileId" in image) && image.bkey) {
      attachments.push({ bkey: image.bkey, remove: true })
    }

    props.onChange(attachments)
  }

  return (
    <div className="flex flex-wrap gap-2.5 mb-4">
      {images.map((image, index) => {
        if ("fileId" in image && !image.file) {
          return (
            <div key={image.fileId} role="alert">
              <p>{image.fileName} is unavailable. Please select it again.</p>
              <Button disabled={props.disabled} onClick={() => remove(image)}>
                Remove image
              </Button>
            </div>
          )
        }

        let preview: File | string | undefined

        if ("fileId" in image) {
          preview = image.file
        } else if (image.upload) {
          preview = `data:${image.upload.contentType};base64,${image.upload.content}`
        } else {
          preview = uploadedImageURL(image.bkey)
        }

        return (
          <ImagePicker
            key={"fileId" in image ? image.fileId : image.bkey || `upload:${index}`}
            field="attachment"
            image={preview}
            disabled={props.disabled}
            onRemove={() => remove(image)}
          />
        )
      })}
      {props.allowUploads && images.length < props.maxUploads && (
        <ImagePicker
          key={`select:${images.length}`}
          field="attachment"
          disabled={props.disabled}
          onSelect={(file) => props.onChange([...props.attachments, commentDrafts.attach(file)])}
        />
      )}
    </div>
  )
}
