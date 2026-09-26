import React from "react"
import { ImageUploader } from "./ImageUploader"
import { ImageUpload } from "@fider/models"
import { ValidationContext, hasError, DisplayError } from "@fider/components"
import { classSet } from "@fider/services"

interface MultiImageUploaderProps {
  field: string
  maxUploads: number
  bkeys?: string[]
  initialUploads?: ImageUpload[]
  disabled?: boolean
  onChange?: (uploads: ImageUpload[]) => void
}

interface ImageEntry {
  id: string
  image: ImageUpload
  read?: Promise<ImageUpload | undefined>
}

interface MultiImageUploaderState {
  entries: ImageEntry[]
}

export class MultiImageUploader extends React.Component<MultiImageUploaderProps, MultiImageUploaderState> {
  private nextID = 0

  constructor(props: MultiImageUploaderProps) {
    super(props)

    const saved = (props.bkeys || []).map((bkey) => ({ bkey, remove: false }))
    const images = [...(props.initialUploads || []), ...saved]

    this.state = {
      entries: images.map((image) => ({
        id: String(this.nextID++),
        image,
      })),
    }
  }

  private imageReading = (read: Promise<ImageUpload | undefined>, instanceID: string) => {
    if (instanceID === String(this.nextID)) {
      this.nextID++
    }

    this.setState((current) => {
      const entries = [...current.entries]
      const index = entries.findIndex((entry) => entry.id === instanceID)

      if (index < 0) {
        entries.push({ id: instanceID, image: { remove: false }, read })
      } else {
        entries[index] = { ...entries[index], read }
      }

      return { entries }
    })
  }

  public async readUploads(): Promise<ImageUpload[] | undefined> {
    const images = await Promise.all(this.state.entries.map((entry) => entry.read || entry.image))

    if (images.some((image) => !image)) {
      return undefined
    }

    return (images as ImageUpload[]).filter((image) => image.upload || image.remove)
  }

  private imageUploaded = (image: ImageUpload, instanceID: string) => {
    if (instanceID === String(this.nextID)) {
      this.nextID++
    }

    this.setState(
      (current) => {
        const entries = [...current.entries]
        const index = entries.findIndex((entry) => entry.id === instanceID)

        if (image.remove && !image.bkey) {
          entries.splice(index, 1)
        } else if (index < 0) {
          entries.push({ id: instanceID, image })
        } else {
          entries[index] = { id: instanceID, image }
        }

        return { entries }
      },
      () => {
        const changes = this.state.entries
          .map((entry) => entry.image)
          .filter((image) => image.upload || image.remove)

        this.props.onChange?.(changes)
      }
    )
  }

  public render() {
    const visible = this.state.entries.filter((entry) => !entry.image.remove)
    const uploaders = visible.map((entry) => (
      <ImageUploader
        key={entry.id}
        instanceID={entry.id}
        field="attachment"
        bkey={entry.image.bkey}
        initialUpload={entry.image}
        disabled={this.props.disabled}
        onRead={(read) => this.imageReading(read, entry.id)}
        onChange={this.imageUploaded}
      />
    ))

    if (visible.length < this.props.maxUploads) {
      const instanceID = String(this.nextID)

      uploaders.push(
        <ImageUploader
          key={instanceID}
          instanceID={instanceID}
          field="attachment"
          disabled={this.props.disabled}
          onRead={(read) => this.imageReading(read, instanceID)}
          onChange={this.imageUploaded}
        />
      )
    }

    return (
      <ValidationContext.Consumer>
        {(ctx) => (
          <div
            className={classSet({
              "mb-4": true,
              "has-error": hasError(this.props.field, ctx.error),
            })}
          >
            <div className="flex flex-wrap gap-2.5">{uploaders}</div>
            <DisplayError fields={[this.props.field]} error={ctx.error} />
          </div>
        )}
      </ValidationContext.Consumer>
    )
  }
}
