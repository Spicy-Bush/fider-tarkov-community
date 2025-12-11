import React from "react"
import { ImageUploader } from "./ImageUploader"
import { ImageUpload } from "@fider/models"
import { ValidationContext, hasError, DisplayError } from "@fider/components"
import { classSet } from "@fider/services"

import "./MultiImageUploader.scss"

interface MultiImageUploaderProps {
  field: string
  maxUploads: number
  bkeys?: string[]
  onChange?: (uploads: ImageUpload[]) => void
}

interface MultiImageUploaderInstances {
  [key: string]: {
    element: JSX.Element
    upload?: ImageUpload
  }
}

interface MultiImageUploaderState {
  count: number
  instances: MultiImageUploaderInstances
  removed: ImageUpload[]
}

export class MultiImageUploader extends React.Component<MultiImageUploaderProps, MultiImageUploaderState> {
  constructor(props: MultiImageUploaderProps) {
    super(props)

    const instances = {}
    if (props.bkeys) {
      for (const bkey of props.bkeys) {
        this.addNewElement(instances, bkey)
      }
    }

    const actualAttachmentCount = this.getActualAttachmentCount(instances, [])
    if (actualAttachmentCount < this.props.maxUploads) {
      this.addNewElement(instances)
    }

    this.state = { instances, count: Object.keys(instances).length, removed: [] }
  }

  private getActualAttachmentCount(instances: MultiImageUploaderInstances, removed: ImageUpload[]): number {
    const currentAttachments = Object.keys(instances)
      .map((k) => instances[k].upload)
      .filter((x) => x && (x.bkey || x.upload) && !x.remove) as ImageUpload[]
    return currentAttachments.length
  }

  private getEmptyUploaderIds(instances: MultiImageUploaderInstances): string[] {
    return Object.keys(instances).filter((k) => {
      const upload = instances[k].upload
      return !upload || (!upload.bkey && !upload.upload && !upload.remove)
    })
  }

  private imageUploaded = (upload: ImageUpload, instanceID: string) => {
    const instances = { ...this.state.instances }
    const removed = [...this.state.removed]
    
    if (upload.remove) {
      if (upload.bkey) {
        removed.push(upload)
      }
      delete instances[instanceID]
    } else {
      instances[instanceID].upload = upload
    }

    const actualAttachmentCount = this.getActualAttachmentCount(instances, removed)
    const emptyUploaderIds = this.getEmptyUploaderIds(instances)

    if (actualAttachmentCount < this.props.maxUploads) {
      if (emptyUploaderIds.length === 0) {
        this.addNewElement(instances)
      }
    } else {
      emptyUploaderIds.forEach((id) => {
        delete instances[id]
      })
    }

    this.setState({ instances, count: Object.keys(instances).length, removed }, this.triggerOnChange)
  }

  private triggerOnChange() {
    if (this.props.onChange) {
      const uploads = Object.keys(this.state.instances)
        .map((k) => this.state.instances[k].upload)
        .concat(this.state.removed)
        .filter((x) => !!x) as ImageUpload[]
      this.props.onChange(uploads)
    }
  }

  private addNewElement(instances: MultiImageUploaderInstances, bkey?: string) {
    const id = btoa(Math.random().toString())
    instances[id] = {
      element: <ImageUploader key={id} bkey={bkey} instanceID={id} field="attachment" onChange={this.imageUploaded} />,
    }
  }

  public render() {
    const elements = Object.keys(this.state.instances).map((k) => this.state.instances[k].element)
    return (
      <ValidationContext.Consumer>
        {(ctx) => (
          <div
            className={classSet({
              "c-form-field": true,
              "c-multi-image-uploader": true,
              "m-error": hasError(this.props.field, ctx.error),
            })}
          >
            <div className="c-multi-image-uploader-instances">{elements}</div>
            <DisplayError fields={[this.props.field]} error={ctx.error} />
          </div>
        )}
      </ValidationContext.Consumer>
    )
  }
}
