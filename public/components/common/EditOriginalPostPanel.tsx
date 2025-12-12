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
    <div className="p-4 px-5">
      <div className="flex items-center gap-3 mb-3 pb-3 border-b border-surface-alt">
        <Button variant="tertiary" size="small" onClick={onCancel}>
          <Icon sprite={IconArrowLeft} className="h-4" />
          <span>Back</span>
        </Button>
        <h4 className="m-0 text-lg font-semibold">Edit Original Post #{post.number}</h4>
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
          <div className="mt-4 p-3 bg-tertiary rounded-card border border-surface-alt">
            <span className="text-xs text-muted uppercase mb-2 block">
              Images from duplicate post ({selectedToAdd.size} selected,{" "}
              {maxImages - currentImageCount - selectedToAdd.size} slots remaining)
            </span>
            <div className="flex flex-wrap gap-2">
              {imagesToTransfer.map((bkey) => {
                const isSelected = selectedToAdd.has(bkey)
                const canSelect = isSelected || canAddMore
                return (
                  <div
                    key={bkey}
                    className={classSet({
                      "relative w-20 h-20 rounded-card overflow-hidden cursor-pointer border-2 border-transparent transition-all duration-50 hover:scale-105 [&_img]:w-full [&_img]:h-full [&_img]:object-cover": true,
                      "border-success": isSelected,
                      "opacity-40 cursor-not-allowed hover:scale-100": !canSelect,
                    })}
                    onClick={() => canSelect && handleToggleImageToAdd(bkey)}
                  >
                    <img src={uploadedImageURL(bkey, 100)} alt="" />
                    {isSelected && (
                      <div className="absolute top-1 right-1 w-5 h-5 bg-success rounded-full flex items-center justify-center text-white">
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
      <div className="p-4 px-5 flex items-center justify-center min-h-[300px] h-full">
        <Loader />
      </div>
    )

    if (variant === "sidebar") {
      return (
        <>
          <div 
            className="fixed inset-0 bg-black/50 z-1099 animate-[fadeIn_0.2s_ease]" 
            onClick={onCancel} 
          />
          <div className="fixed top-0 right-0 bottom-0 w-full max-w-[600px] z-1100 bg-elevated shadow-xl animate-[slideInFromRight_0.2s_ease] overflow-y-auto max-sm:max-w-full max-sm:left-0">
            {loadingContent}
          </div>
        </>
      )
    }

    return (
      <div 
        className="absolute inset-0 z-base bg-elevated overflow-y-auto rounded-panel animate-[slideIn_0.2s_ease] max-lg:fixed max-lg:rounded-none" 
        onClick={handleBackdropClick}
      >
        {loadingContent}
      </div>
    )
  }

  if (variant === "sidebar") {
    return (
      <>
        <div 
          className="fixed inset-0 bg-black/50 z-1099 animate-[fadeIn_0.2s_ease]" 
          onClick={onCancel} 
        />
        <div className="fixed top-0 right-0 bottom-0 w-full max-w-[600px] z-1100 bg-elevated shadow-xl animate-[slideInFromRight_0.2s_ease] overflow-y-auto max-sm:max-w-full max-sm:left-0">
          {panelContent}
        </div>
      </>
    )
  }

  return (
    <div 
      className="absolute inset-0 z-base bg-elevated overflow-y-auto rounded-panel animate-[slideIn_0.2s_ease] max-lg:fixed max-lg:rounded-none" 
      onClick={handleBackdropClick}
    >
      {panelContent}
    </div>
  )
}
