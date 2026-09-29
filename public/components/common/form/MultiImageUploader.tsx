import React from "react"
import { Button } from "../Button"
import { ImagePicker } from "./ImagePicker"
import { DraftImage } from "@fider/services/draftImages"
import { newSubmissionID } from "@fider/services/postSubmission"
import { uploadedImageURL } from "@fider/services/utils"

interface MultiImageUploaderProps {
  field: string
  maxUploads: number
  value: DraftImage[]
  disabled?: boolean
  allowUploads?: boolean
  onChange: (images: DraftImage[]) => void
}

export function MultiImageUploader(props: MultiImageUploaderProps) {
  const images = props.value.filter(image => image.kind !== "removed")

  const remove = (image: DraftImage) => {
    const remaining = props.value.filter(item => item !== image)
    const storedKey = image.kind === "stored" ? image.bkey : image.kind === "local" ? image.replaces : undefined
    if (storedKey) remaining.push({ kind: "removed", bkey: storedKey })
    props.onChange(remaining)
  }

  return (
    <div className="flex flex-wrap gap-2.5 mb-4">
      {images.map((image) => {
        if (image.kind === "missing") {
          return (
            <div key={image.fileId} className="rounded-card border border-warning p-3 text-sm">
              <p role="alert" className="mb-2">{image.fileName} is unavailable. Select it again or remove it.</p>
              <div className="flex items-start gap-2">
                <ImagePicker field={props.field} disabled={props.disabled} onSelect={file => {
                  const id = newSubmissionID()
                  props.onChange(props.value.map(item => item === image ? {
                    kind: "local",
                    fileId: id,
                    file,
                  } : item))
                }} />
                <Button disabled={props.disabled} onClick={() => remove(image)}>Remove image</Button>
              </div>
            </div>
          )
        }

        const preview = image.kind === "local" ? image.file : uploadedImageURL(image.bkey)
        return (
          <ImagePicker
            key={image.kind === "local" ? image.fileId : image.bkey}
            field={props.field}
            image={preview}
            disabled={props.disabled}
            onRemove={() => remove(image)}
          />
        )
      })}
      {props.allowUploads !== false && images.length < props.maxUploads && (
        <ImagePicker field={props.field} disabled={props.disabled} onSelect={file => {
          const id = newSubmissionID()
          props.onChange([...props.value, { kind: "local", fileId: id, file }])
        }} />
      )}
    </div>
  )
}
