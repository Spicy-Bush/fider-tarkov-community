import React, { useState, useEffect, useRef } from "react"
import { Button, ButtonClickEvent, Input, Form, TextArea, MultiImageUploader } from "@fider/components"
import { SignInModal } from "@fider/components"
import { PreviewPostModal } from "./PreviewPostModal"
import { cache, actions, Failure } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
// import "./PostInput.scss"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface PostInputProps {
  placeholder: string
  onTitleChanged: (title: string) => void
}

const CACHE_TITLE_KEY = "PostInput-Title"
const CACHE_DESCRIPTION_KEY = "PostInput-Description"

export const PostInput = (props: PostInputProps) => {
  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const titleRef = useRef<HTMLInputElement>()
  const timerRef = useRef<NodeJS.Timeout | null>(null)
  
  const getCachedValue = (key: string): string => 
    fider.session.isAuthenticated ? (cache.session.get(key) || "") : ""
  
  const [title, setTitle] = useState(getCachedValue(CACHE_TITLE_KEY))
  const [description, setDescription] = useState(getCachedValue(CACHE_DESCRIPTION_KEY))
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [error, setError] = useState<Failure | undefined>(undefined)
  const [isPendingSubmission, setIsPendingSubmission] = useState(false)
  const [remainingSeconds, setRemainingSeconds] = useState(30)
  const [isPreviewModalOpen, setIsPreviewModalOpen] = useState(false)
  
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
    isMuted ||
    (fider.session.user?.role !== "administrator" && 
     (postingGloballyDisabled || 
      settings.postingDisabledFor?.includes(fider.session.user?.role || "") || 
      false))

  useEffect(() => {
    props.onTitleChanged(title)
  }, [title])
  
  useEffect(() => {
    let intervalId: NodeJS.Timeout | null = null;
    
    if (isPendingSubmission && remainingSeconds > 0) {
      intervalId = setInterval(() => {
        setRemainingSeconds(prev => {
          if (prev <= 1) {
            if (intervalId) clearInterval(intervalId);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    } else if (isPendingSubmission && remainingSeconds === 0) {
      const event = {} as ButtonClickEvent;
      submitPost(event);
    }

    return () => {
      if (intervalId) clearInterval(intervalId);
    };
  }, [isPendingSubmission, remainingSeconds]);
  
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
  
  const submitPost = async (event: ButtonClickEvent) => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    
    const result = await actions.createPost(title, description, attachments)
    setIsPendingSubmission(false)
    setRemainingSeconds(30)
    
    if (result.ok) {
      setError(undefined)
      cache.session.remove(CACHE_TITLE_KEY, CACHE_DESCRIPTION_KEY)
      location.href = `/posts/${result.data.number}/${result.data.slug}`
      event.preventEnable()
    } else if (result.error) {
      setError(result.error)
    }
  }

  const submit = async (event: ButtonClickEvent) => {
    if (title) {
      setRemainingSeconds(30)
      setIsPendingSubmission(true)
    }
  }
  
  const submitNow = (event: ButtonClickEvent) => {
    submitPost(event)
  }
  
  const cancelSubmission = () => {
    setIsPendingSubmission(false)
    setRemainingSeconds(30)
  }
  
  const showPreview = () => {
    setIsPreviewModalOpen(true)
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

  const progressPercentage = ((30 - remainingSeconds) / 30) * 100

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
      <Form error={error}>
        {isPostingDisabled && fider.session.isAuthenticated && (
          <div className="p-3 bg-warning/10 border border-warning rounded text-warning">
            {isMuted ? (
              <Trans id="home.postinput.muted">
                You are currently muted. Reason: {muteReason}
              </Trans>
            ) : (
              <Trans id="home.postinput.disabled">
                Posting has been disabled by the administrators.
              </Trans>
            )}
          </div>
        )}
        {!isPostingDisabled && (
          <>
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
                <div className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-muted">
                  {titleLengthMin - title.length}
                </div>
              )}
              {titleValidation.showMaxCounter && (
                <div className={`absolute right-2 top-1/2 -translate-y-1/2 text-xs ${titleValidation.isOverMax ? 'text-danger' : 'text-muted'}`}>
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
                    <div className="absolute right-2 bottom-2 text-xs text-muted">
                      {descriptionLengthMin - description.length}
                    </div>
                  )}
                  {descValidation.showMaxCounter && (
                    <div className={`absolute right-2 bottom-2 text-xs ${descValidation.isOverMax ? 'text-danger' : 'text-muted'}`}>
                      {descriptionLengthMax - description.length}
                    </div>
                  )}
                </div>
                <MultiImageUploader field="attachments" maxUploads={maxImagesPerPost} onChange={setAttachments} />
                
                {isPendingSubmission ? (
                  <div className="flex justify-between items-center">
                    <div className="flex gap-2">
                      <Button type="button" variant="secondary" onClick={submitNow}>
                        <Trans id="action.sendnow">Send Now</Trans>
                      </Button>
                      <Button type="button" variant="danger" onClick={cancelSubmission}>
                        <Trans id="action.cancel">Cancel</Trans>
                      </Button>
                    </div>
                    
                    <div className="flex items-center">
                      <div className="mr-2 inline-block">
                        <svg width="12" height="12" viewBox="0 0 24 24">
                          <circle 
                            cx="12" cy="12" r="10" 
                            fill="none" stroke="currentColor" strokeWidth="4" 
                            strokeDasharray={Math.PI * 2 * 10}
                            strokeDashoffset={(Math.PI * 2 * 10) * (1 - progressPercentage / 100)}
                            transform="rotate(-90 12 12)"
                          />
                        </svg>
                      </div>
                      <span className="text-sm text-muted">
                        <Trans id="action.submitting">Submitting in {remainingSeconds}s...</Trans>
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className="flex justify-between items-center">
                    <Button type="submit" variant="primary" disabled={isSubmitDisabled} onClick={submit}>
                      <Trans id="action.submit">Submit</Trans>
                    </Button>
                    <Button type="button" variant="secondary" disabled={isSubmitDisabled} onClick={showPreview}>
                      <Trans id="action.preview">Preview</Trans>
                    </Button>
                  </div>
                )}
              </>
            )}
          </>
        )}
      </Form>
      
      <PreviewPostModal
        isOpen={isPreviewModalOpen}
        title={title}
        description={description}
        attachments={attachments}
        onClose={() => setIsPreviewModalOpen(false)}
      />
    </>
  )
}
