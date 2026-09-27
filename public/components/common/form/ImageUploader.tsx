import React from "react"
import { fileToBase64, uploadedImageURL } from "@fider/services"
import { Button } from "@fider/components"
import { ImageUpload } from "@fider/models"
import { ImagePicker } from "./ImagePicker"

interface ImageUploaderProps {
  children?: React.ReactNode
  instanceID?: string
  field: string
  label?: string
  bkey?: string
  previewURL?: string
  initialUpload?: ImageUpload
  disabled?: boolean
  onRead?: (image: Promise<ImageUpload | undefined>) => void
  onChange(state: ImageUpload, instanceID?: string, previewURL?: string): void
}

interface ImageUploaderState extends ImageUpload {
  reading?: boolean
  failedFile?: File
}

export class ImageUploader extends React.Component<ImageUploaderProps, ImageUploaderState> {
  private currentRead?: Promise<ImageUpload | undefined>

  constructor(props: ImageUploaderProps) {
    super(props)
    this.state = { upload: undefined, remove: false, ...props.initialUpload }
  }

  private get previewURL() {
    if (this.state.remove) {
      return undefined
    }

    const upload = this.state.upload

    if (upload) {
      return `data:${upload.contentType};base64,${upload.content}`
    }

    return this.props.previewURL ?? uploadedImageURL(this.props.bkey)
  }

  private readFile = (file: File) => {
    const read = fileToBase64(file).then(
      (content): ImageUpload => ({
        bkey: this.props.bkey,
        upload: {
          fileName: file.name,
          content,
          contentType: file.type,
        },
        remove: false,
      }),
      () => undefined
    )

    this.currentRead = read
    this.setState({ reading: true, failedFile: undefined })
    this.props.onRead?.(read)

    void read.then((image) => {
      if (this.currentRead !== read) {
        return
      }

      if (!image) {
        this.setState({ reading: false, failedFile: file })
        return
      }

      this.setState({ ...image, reading: false }, () => {
        this.props.onChange(image, this.props.instanceID, this.previewURL)
      })
    })
  }

  public componentWillUnmount() {
    this.currentRead = undefined
  }

  private removeFile = () => {
    this.currentRead = undefined
    const image: ImageUpload = { bkey: this.props.bkey, remove: true }

    this.setState({ ...image, upload: undefined, reading: false, failedFile: undefined }, () => {
      this.props.onChange(image, this.props.instanceID, this.previewURL)
    })
  }

  public render() {
    return (
      <ImagePicker
        field={this.props.field}
        label={this.props.label}
        image={this.previewURL}
        disabled={this.props.disabled}
        reading={this.state.reading}
        onSelect={this.state.failedFile ? undefined : this.readFile}
        onRemove={this.removeFile}
      >
        {this.state.failedFile && (
          <div role="alert">
            <p>Could not read {this.state.failedFile.name}.</p>
            <Button onClick={() => this.readFile(this.state.failedFile!)} disabled={this.props.disabled}>
              Retry image
            </Button>
            <Button onClick={this.removeFile} disabled={this.props.disabled}>
              Remove image
            </Button>
          </div>
        )}
        {this.props.children}
      </ImagePicker>
    )
  }
}
