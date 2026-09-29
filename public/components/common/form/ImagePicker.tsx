import React, { useContext, useId, useLayoutEffect, useRef, useState } from "react"
import { Button } from "@fider/components/common/Button"
import { Icon } from "@fider/components/common/Icon"
import { Modal } from "@fider/components/common/Modal"
import { classSet } from "@fider/services/utils"
import { heroiconsPhotograph as IconPhotograph } from "@fider/icons.generated"
import { ValidationContext } from "./Form"
import { DisplayError, hasError } from "./DisplayError"

const hardFileSizeLimit = 7.5 * 1024 * 1024

interface ImagePickerProps {
  field: string
  label?: string
  image?: File | string
  disabled?: boolean
  reading?: boolean
  children?: React.ReactNode
  onSelect?: (file: File) => void
  onRemove?: () => void
}

export function ImagePicker(props: ImagePickerProps) {
  const validation = useContext(ValidationContext)
  const input = useRef<HTMLInputElement>(null)
  const inputId = useId()
  const [filePreview, setFilePreview] = useState<string>()
  const [showModal, setShowModal] = useState(false)

  useLayoutEffect(() => {
    if (!props.image || typeof props.image === "string") {
      setFilePreview(undefined)
      return
    }

    const preview = URL.createObjectURL(props.image)
    setFilePreview(preview)

    return () => URL.revokeObjectURL(preview)
  }, [props.image])

  const preview = typeof props.image === "string" ? props.image : filePreview

  const select = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]

    if (!file) {
      return
    }

    if (file.size > hardFileSizeLimit) {
      alert("The image size must be smaller than 7MB.")
      return
    }

    props.onSelect?.(file)
    event.target.value = ""
  }

  return (
    <div className={classSet({ "mb-4": true, "has-error": hasError(props.field, validation.error) })}>
      <Modal.Window isOpen={showModal} onClose={() => setShowModal(false)} center={false} size="fluid">
        <Modal.Content>
          <img alt="" src={preview} />
        </Modal.Content>
        <Modal.Footer>
          <Button variant="tertiary" onClick={() => setShowModal(false)}>
            Close
          </Button>
        </Modal.Footer>
      </Modal.Window>

      {props.label && (
        <label htmlFor={inputId} className="block text-sm font-medium mb-1">
          {props.label}
        </label>
      )}

      {props.image && (
        <div className="relative inline-block h-20">
          <button
            type="button"
            aria-label="Preview image"
            onClick={() => setShowModal(true)}
            className="h-full cursor-pointer"
          >
            <img alt="" src={preview} className="p-1 min-w-[50px] min-h-[50px] border border-border h-full" />
          </button>
          {!props.disabled && props.onRemove && (
            <Button
              onClick={() => props.onRemove?.()}
              variant="danger"
              className="absolute top-1 right-1 h-7 w-7 justify-center rounded-full !p-0"
            >
              <span aria-hidden="true">X</span>
              <span className="sr-only">Remove image</span>
            </Button>
          )}
        </div>
      )}

      <input
        ref={input}
        id={inputId}
        type="file"
        disabled={props.disabled}
        onChange={select}
        accept="image/png, image/jpeg, image/jpg, image/webp"
        className="hidden"
      />
      {!props.image && props.onSelect && (
        <Button variant="secondary" onClick={() => input.current?.click()} disabled={props.disabled} loading={props.reading}>
          <Icon sprite={IconPhotograph} />
          <span className="sr-only">Select image</span>
        </Button>
      )}
      {props.children}
      <DisplayError fields={[props.field]} error={validation.error} />
    </div>
  )
}
