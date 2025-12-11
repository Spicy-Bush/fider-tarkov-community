import React, { useState, useEffect } from "react"
import {
  Button,
  Loader,
  Icon,
  Input,
  TextArea,
  Form,
  MultiImageUploader,
} from "@fider/components"
import { HStack } from "@fider/components/layout"
import { actions, Fider, Failure, classSet, uploadedImageURL } from "@fider/services"
import {
  heroiconsArrowLeft as IconArrowLeft,
  heroiconsCheck as IconCheck,
} from "@fider/icons.generated"
import { Post, ImageUpload } from "@fider/models"

import "./EditOriginalPostPanel.scss"

export type EditOriginalPostPanelVariant = "overlay" | "sidebar"

export interface EditOriginalPostPanelProps {
  post: Post
  attachments: string[]
  imagesToTransfer: string[]
  isLoading?: boolean
  variant?: EditOriginalPostPanelVariant
  onSave: (post: Post) => void
  onCancel: () => void
}

export const EditOriginalPostPanel: React.FC<EditOriginalPostPanelProps> = ({
  post,
  attachments,
  imagesToTransfer,
  isLoading = false,
  variant = "overlay",
  onSave,
  onCancel,
}) => {
  const maxImages = Fider.session.tenant.generalSettings?.maxImagesPerPost || 3
  const [title, setTitle] = useState(post.title)
  const [description, setDescription] = useState(post.description || "")
  const [existingBkeys, setExistingBkeys] = useState<string[]>(attachments)
  const [newAttachments, setNewAttachments] = useState<ImageUpload[]>([])
  const [selectedToAdd, setSelectedToAdd] = useState<Set<string>>(new Set())
  const [error, setError] = useState<Failure | undefined>()
  const [isSaving, setIsSaving] = useState(false)

  const currentImageCount = existingBkeys.length + newAttachments.filter((a) => !a.remove).length
  const canAddMore = currentImageCount + selectedToAdd.size < maxImages

  useEffect(() => {
    setTitle(post.title)
    setDescription(post.description || "")
  }, [post.number, post.title, post.description])

  useEffect(() => {
    setExistingBkeys(attachments)
    setNewAttachments([])
    setSelectedToAdd(new Set())
  }, [attachments])

  const handleToggleImageToAdd = (bkey: string) => {
    setSelectedToAdd((prev) => {
      const newSet = new Set(prev)
      if (newSet.has(bkey)) {
        newSet.delete(bkey)
      } else if (currentImageCount + newSet.size < maxImages) {
        newSet.add(bkey)
      }
      return newSet
    })
  }

  const handleSave = async () => {
    setIsSaving(true)
    setError(undefined)

    const selectedToAddImages: ImageUpload[] = Array.from(selectedToAdd).map((bkey) => ({ bkey, remove: false }))
    
    const allAttachments: ImageUpload[] = [
      ...newAttachments,
      ...selectedToAddImages,
    ]

    const result = await actions.updatePost(post.number, title, description, allAttachments)
    if (result.ok) {
      onSave({ ...post, title, description })
    } else {
      setError(result.error)
    }
    setIsSaving(false)
  }

  const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      onCancel()
    }
  }

  const panelContent = (
    <div className="c-edit-original-panel">
      <div className="c-edit-original-panel__header">
        <Button variant="tertiary" size="small" onClick={onCancel}>
          <Icon sprite={IconArrowLeft} className="h-4" />
          <span>Back</span>
        </Button>
        <h4>Edit Original Post #{post.number}</h4>
      </div>
      <p className="text-muted mb-4">
        You copied content from the duplicate post. Edit the original post below to merge
        the content, then save to proceed with marking as duplicate.
      </p>
      <Form error={error}>
        <Input
          field="title"
          label="Title"
          maxLength={100}
          value={title}
          onChange={setTitle}
        />
        <TextArea
          field="description"
          label="Description"
          value={description}
          onChange={setDescription}
          minRows={8}
        />
        <MultiImageUploader
          field="attachments"
          bkeys={existingBkeys}
          maxUploads={maxImages}
          onChange={setNewAttachments}
        />
        {imagesToTransfer.length > 0 && (
          <div className="c-edit-original-panel__images-to-add">
            <span className="text-xs text-muted uppercase mb-2 block">
              Images from duplicate post ({selectedToAdd.size} selected,{" "}
              {maxImages - currentImageCount - selectedToAdd.size} slots remaining)
            </span>
            <div className="c-edit-original-panel__image-grid">
              {imagesToTransfer.map((bkey) => {
                const isSelected = selectedToAdd.has(bkey)
                const canSelect = isSelected || canAddMore
                return (
                  <div
                    key={bkey}
                    className={classSet({
                      "c-edit-original-panel__image-item": true,
                      "c-edit-original-panel__image-item--selected": isSelected,
                      "c-edit-original-panel__image-item--disabled": !canSelect,
                    })}
                    onClick={() => canSelect && handleToggleImageToAdd(bkey)}
                  >
                    <img src={uploadedImageURL(bkey, 100)} alt="" />
                    {isSelected && (
                      <div className="c-edit-original-panel__image-check">
                        <Icon sprite={IconCheck} className="h-4" />
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )}
        <HStack spacing={2} className="mt-4">
          <Button variant="primary" onClick={handleSave} disabled={isSaving}>
            <Icon sprite={IconCheck} className="h-4" />
            <span>{isSaving ? "Saving..." : "Save & Continue"}</span>
          </Button>
          <Button variant="tertiary" onClick={onCancel} disabled={isSaving}>
            <span>Skip Edit</span>
          </Button>
        </HStack>
      </Form>
    </div>
  )

  if (isLoading) {
    const loadingContent = (
      <div className="c-edit-original-panel c-edit-original-panel--loading">
        <Loader />
      </div>
    )

    if (variant === "sidebar") {
      return (
        <>
          <div className="c-edit-original-backdrop" onClick={onCancel} />
          <div className="c-edit-original-sidebar">{loadingContent}</div>
        </>
      )
    }

    return (
      <div className="c-edit-original-overlay" onClick={handleBackdropClick}>
        {loadingContent}
      </div>
    )
  }

  if (variant === "sidebar") {
    return (
      <>
        <div className="c-edit-original-backdrop" onClick={onCancel} />
        <div className="c-edit-original-sidebar">{panelContent}</div>
      </>
    )
  }

  return (
    <div className="c-edit-original-overlay" onClick={handleBackdropClick}>
      {panelContent}
    </div>
  )
}

