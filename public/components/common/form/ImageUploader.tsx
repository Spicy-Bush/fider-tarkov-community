import React from "react"
import { ValidationContext } from "./Form"
import { DisplayError, hasError } from "./DisplayError"
import { classSet, fileToBase64, uploadedImageURL } from "@fider/services"
import { Button, Icon, Modal } from "@fider/components"
import { ImageUpload } from "@fider/models"
import { heroiconsPhotograph as IconPhotograph } from "@fider/icons.generated"

const hardFileSizeLimit = 7.5 * 1024 * 1024

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
  showModal: boolean
  reading?: boolean
  failedFile?: File
}

export class ImageUploader extends React.Component<ImageUploaderProps, ImageUploaderState> {
  private fileSelector?: HTMLInputElement | null
  private currentRead?: Promise<ImageUpload | undefined>

  constructor(props: ImageUploaderProps) {
    super(props)
    this.state = {
      upload: undefined,
      remove: false,
      ...props.initialUpload,
      showModal: false,
    }
  }

  private get previewURL() {
    if (this.state.remove) return undefined
    const upload = this.state.upload
    return upload ? `data:${upload.contentType};base64,${upload.content}` : (this.props.previewURL ?? uploadedImageURL(this.props.bkey))
  }

  public fileChanged = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0]
      if (file.size > hardFileSizeLimit) {
        alert("The image size must be smaller than 7MB.")
        return
      }

      this.readFile(file)
    }
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

  public removeFile = async () => {
    this.currentRead = undefined

    if (this.fileSelector) {
      this.fileSelector.value = ""
    }

    this.setState(
      {
        bkey: this.props.bkey,
        remove: true,
        upload: undefined,
        reading: false,
        failedFile: undefined,
      },
      () => {
        this.props.onChange(
          {
            bkey: this.state.bkey,
            remove: this.state.remove,
            upload: this.state.upload,
          },
          this.props.instanceID,
          this.previewURL
        )
      }
    )
  }

  public selectFile = async () => {
    if (this.fileSelector) {
      this.fileSelector.click()
    }
  }

  private openModal = () => {
    this.setState({ showModal: true })
  }

  private closeModal = async () => {
    this.setState({ showModal: false })
  }

  private modal() {
    return (
      <Modal.Window isOpen={this.state.showModal} onClose={this.closeModal} center={false} size="fluid">
        <Modal.Content>{<img alt="" src={this.previewURL} />}</Modal.Content>

        <Modal.Footer>
          <Button variant="tertiary" onClick={this.closeModal}>
            Close
          </Button>
        </Modal.Footer>
      </Modal.Window>
    )
  }

  public render() {
    const isUploading = !!this.state.upload
    const hasFile = (!this.state.remove && (this.props.bkey || this.props.previewURL)) || isUploading

    return (
      <ValidationContext.Consumer>
        {(ctx) => (
          <div
            className={classSet({
              "mb-4": true,
              "has-error": hasError(this.props.field, ctx.error),
            })}
          >
            {this.modal()}
            {this.props.label && (
              <label htmlFor={`input-${this.props.field}`} className="block text-sm font-medium mb-1">
                {this.props.label}
              </label>
            )}

            {hasFile && (
              <div className="relative inline-block h-20">
                <img
                  alt=""
                  onClick={this.openModal}
                  src={this.previewURL}
                  className="p-1 min-w-[50px] min-h-[50px] border border-border cursor-pointer h-full"
                />
                {!this.props.disabled && (
                  <Button onClick={this.removeFile} variant="danger" className="absolute top-1 right-1 rounded-full px-1.5 py-1">
                    X
                  </Button>
                )}
              </div>
            )}

            <input
              ref={(e) => (this.fileSelector = e)}
              type="file"
              onChange={this.fileChanged}
              accept="image/png, image/jpeg, image/jpg, image/webp"
              className="hidden"
            />
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
            {!hasFile && !this.state.failedFile && (
              <Button variant="secondary" onClick={this.selectFile} disabled={this.props.disabled} loading={this.state.reading}>
                <Icon sprite={IconPhotograph} />
              </Button>
            )}
            <DisplayError fields={[this.props.field]} error={ctx.error} />
            {this.props.children}
          </div>
        )}
      </ValidationContext.Consumer>
    )
  }
}
