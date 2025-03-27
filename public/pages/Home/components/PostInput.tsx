import React, { useState, useEffect, useRef } from "react"
import { Button, ButtonClickEvent, Input, Form, TextArea, MultiImageUploader } from "@fider/components"
import { SignInModal } from "@fider/components"
import { cache, actions, Failure } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

interface PostInputProps {
  placeholder: string
  onTitleChanged: (title: string) => void
}

const CACHE_TITLE_KEY = "PostInput-Title"
const CACHE_DESCRIPTION_KEY = "PostInput-Description"

export const PostInput = (props: PostInputProps) => {
  const fider = useFider()
  const titleRef = useRef<HTMLInputElement>()
  
  const getCachedValue = (key: string): string => 
    fider.session.isAuthenticated ? (cache.session.get(key) || "") : ""
  
  const [title, setTitle] = useState(getCachedValue(CACHE_TITLE_KEY))
  const [description, setDescription] = useState(getCachedValue(CACHE_DESCRIPTION_KEY))
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [error, setError] = useState<Failure | undefined>(undefined)
  
  const settings = fider.session.tenant.generalSettings || {
    titleLengthMin: 15,
    titleLengthMax: 100,
    descriptionLengthMin: 150,
    descriptionLengthMax: 1000,
    maxImagesPerPost: 3,
    maxImagesPerComment: 2,
    postLimits: {} as Record<string, { count: number; hours: number }>,
    commentLimits: {} as Record<string, { count: number; hours: number }>,
    postingDisabledFor: [] as string[],
    commentingDisabledFor: [] as string[],
    postingGloballyDisabled: false,
    commentingGloballyDisabled: false
  }
  
  const { 
    titleLengthMin, 
    titleLengthMax, 
    descriptionLengthMin, 
    descriptionLengthMax, 
    postingGloballyDisabled,
    maxImagesPerPost
  } = settings
  
  const isPostingDisabled = !fider.session.isAuthenticated || 
    (fider.session.user?.role !== "administrator" && 
     (postingGloballyDisabled || 
      settings.postingDisabledFor?.includes(fider.session.user?.role || "") || 
      false))

  useEffect(() => {
    props.onTitleChanged(title)
  }, [title])

  const handleTitleFocus = () => {
    if (!fider.session.isAuthenticated && titleRef.current) {
      titleRef.current.blur()
      setIsSignInModalOpen(true)
    }
  }

  const handleTitleChange = (value: string) => {
    cache.session.set(CACHE_TITLE_KEY, value)
    setTitle(value)
    props.onTitleChanged(value)
  }

  const handleDescriptionChange = (value: string) => {
    cache.session.set(CACHE_DESCRIPTION_KEY, value)
    setDescription(value)
  }

  const submit = async (event: ButtonClickEvent) => {
    if (title) {
      const result = await actions.createPost(title, description, attachments)
      if (result.ok) {
        setError(undefined)
        cache.session.remove(CACHE_TITLE_KEY, CACHE_DESCRIPTION_KEY)
        location.href = `/posts/${result.data.number}/${result.data.slug}`
        event.preventEnable()
      } else if (result.error) {
        setError(result.error)
      }
    }
  }
  
  const titleValidation = {
    showMinCounter: title.length > 0 && title.length < titleLengthMin,
    showMaxCounter: title.length >= titleLengthMax * 0.9,
    isOverMax: title.length > titleLengthMax
  }
  
  const descValidation = {
    showMinCounter: description.length > 0 && description.length < descriptionLengthMin,
    showMaxCounter: description.length >= descriptionLengthMax * 0.9,
    isOverMax: description.length > descriptionLengthMax
  }

  const isSubmitDisabled = title.length < titleLengthMin || 
                           titleValidation.isOverMax || 
                           (description.length > 0 && description.length < descriptionLengthMin) || 
                           descValidation.isOverMax || 
                           isPostingDisabled

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
      <Form error={error}>
        {isPostingDisabled && fider.session.isAuthenticated && (
          <div className="c-message c-message--warning">
            <Trans id="home.postinput.disabled">
              Posting has been disabled by the administrators.
            </Trans>
          </div>
        )}
        <div className="relative">
          <Input
            field="title"
            disabled={fider.isReadOnly || isPostingDisabled}
            noTabFocus={!fider.session.isAuthenticated}
            inputRef={titleRef}
            onFocus={handleTitleFocus}
            maxLength={titleLengthMax}
            value={title}
            onChange={handleTitleChange}
            placeholder={props.placeholder}
          />
          {titleValidation.showMinCounter && (
            <div className="c-input-counter c-input-counter--min">
              {titleLengthMin - title.length}
            </div>
          )}
          {titleValidation.showMaxCounter && (
            <div className={`c-input-counter c-input-counter--max ${titleValidation.isOverMax ? 'c-input-counter--error' : ''}`}>
              {titleLengthMax - title.length}
            </div>
          )}
        </div>
        {title && (
          <>
            <div className="relative">
              <TextArea
                field="description"
                onChange={handleDescriptionChange}
                value={description}
                minRows={5}
                disabled={isPostingDisabled}
                placeholder={i18n._("home.postinput.description.placeholder", { message: "Describe your suggestion..." })}
              />
              {descValidation.showMinCounter && (
                <div className="c-input-counter c-input-counter--min">
                  {descriptionLengthMin - description.length}
                </div>
              )}
              {descValidation.showMaxCounter && (
                <div className={`c-input-counter c-input-counter--max ${descValidation.isOverMax ? 'c-input-counter--error' : ''}`}>
                  {descriptionLengthMax - description.length}
                </div>
              )}
            </div>
            <MultiImageUploader field="attachments" maxUploads={maxImagesPerPost} onChange={setAttachments} />
            <Button 
              type="submit" 
              variant="primary" 
              disabled={isSubmitDisabled} 
              onClick={submit}
            >
              <Trans id="action.submit">Submit</Trans>
            </Button>
          </>
        )}
      </Form>
    </>
  )
}
