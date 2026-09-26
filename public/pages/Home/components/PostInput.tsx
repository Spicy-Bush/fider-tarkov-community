import React, { useState, useEffect, useRef } from "react"
import { Button, Input, Form, TextArea, MultiImageUploader } from "@fider/components"
import { PreviewPostModal } from "./PreviewPostModal"
import { SavedPostRecovery } from "./SavedPostRecovery"
import { cache, Failure } from "@fider/services"
import { postSubmissions, PendingPostSubmission, newSubmissionID, sendPostSubmission, SubmissionStorageError } from "@fider/services/postSubmission"
import { RequestError } from "@fider/services/http"
import { CurrentUser, ImageUpload, Post } from "@fider/models"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface PostInputProps {
  placeholder: string
  onTitleChanged: (title: string) => void
}

type SubmissionState =
  | { phase: "loading" | "load-failed" | "idle" | "countdown" | "collecting" | "complete" }
  | { phase: "preparing" | "sending" | "unconfirmed"; submission: PendingPostSubmission }

export const PostInput = (props: PostInputProps) => {
  const fider = useFider()

  if (!fider.session.isAuthenticated) {
    return null
  }

  const user = fider.session.user
  const account = `${fider.session.tenant.id}:${user.id}`

  return <AuthenticatedPostInput key={account} {...props} account={account} user={user} />
}

const AuthenticatedPostInput = (props: PostInputProps & { account: string; user: CurrentUser }) => {
  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const { account, user } = props
  const titleCacheKey = "PostInput-Title"
  const descriptionCacheKey = "PostInput-Description"
  const submissionCacheKey = "PostInput-Submission"
  const editingCacheKey = "PostInput-Editing"
  const [requestedSubmissionId] = useState(() => {
    const search = window.location.search

    return search ? new URLSearchParams(search).get("submission") : null
  })
  const uploader = useRef<MultiImageUploader>(null)
  const [otherSubmissions, setOtherSubmissions] = useState<PendingPostSubmission[]>([])
  const [state, setState] = useState<SubmissionState>({ phase: "loading" })
  const phase = state.phase
  const [loadVersion, setLoadVersion] = useState(0)
  
  const [title, setTitle] = useState(() => cache.session.get(titleCacheKey) || "")
  const [description, setDescription] = useState(() => cache.session.get(descriptionCacheKey) || "")
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [error, setError] = useState<Failure | undefined>(undefined)
  const isPendingSubmission = phase === "countdown"
  const isSending = phase === "collecting" || phase === "preparing" || phase === "sending" || phase === "complete"
  const isFrozen = phase !== "idle" && phase !== "countdown"
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
  
  const isPostingDisabled = isMuted || (
    user.role !== "administrator" && (
      postingGloballyDisabled || settings.postingDisabledFor?.includes(user.role)
    )
  )

  const finishSubmission = (receipt: Pick<Post, "number" | "slug">) => {
    cache.session.remove(titleCacheKey, descriptionCacheKey, submissionCacheKey, editingCacheKey)
    setState({ phase: "complete" })
    location.href = `/posts/${receipt.number}/${receipt.slug}`
  }

  useEffect(() => {
    props.onTitleChanged(title)
  }, [title])
  
  useEffect(() => {
    let mounted = true
    setState({ phase: "loading" })

    postSubmissions.load(account).then((saved) => {
      if (!mounted) {
        return
      }

      const submissionId = requestedSubmissionId || cache.session.get(submissionCacheKey)
      const own = saved.find((item) => item.submissionId === submissionId)

      setOtherSubmissions(saved.filter((item): item is PendingPostSubmission => !("receipt" in item) && item !== own))

      if (own && "receipt" in own) {
        finishSubmission(own.receipt)
      } else if (own) {
        const editing = !!requestedSubmissionId || !!own.rejection || cache.session.get(editingCacheKey) === own.submissionId

        setTitle(editing && !requestedSubmissionId ? cache.session.get(titleCacheKey) ?? own.title : own.title)
        setDescription(editing && !requestedSubmissionId ? cache.session.get(descriptionCacheKey) ?? own.description : own.description)
        setAttachments(own.attachments)
        cache.session.set(submissionCacheKey, own.submissionId)

        if (editing) {
          if (requestedSubmissionId) {
            cache.session.set(titleCacheKey, own.title)
            cache.session.set(descriptionCacheKey, own.description)
          }

          cache.session.set(editingCacheKey, own.submissionId)
          setState({ phase: "idle" })
          history.replaceState(null, "", "/")
        } else {
          setState({ phase: "sending", submission: own })
        }
      } else {
        setState({ phase: "idle" })
      }
    }).catch((cause) => {
      if (!(cause instanceof SubmissionStorageError)) {
        throw cause
      }

      if (mounted) {
        setState({ phase: "load-failed" })
        setError({ errors: [{ message: "Your browser could not load saved submissions." }] })
      }
    })

    return () => {
      mounted = false
    }
  }, [account, loadVersion])

  useEffect(() => {
    if (!isPendingSubmission) {
      return
    }

    if (remainingSeconds === 0) {
      void submitPost()
      return
    }

    const timer = setTimeout(() => setRemainingSeconds(remainingSeconds - 1), 1000)

    return () => clearTimeout(timer)
  }, [phase, remainingSeconds])

  const handleTitleChange = (value: string) => {
    cache.session.set(titleCacheKey, value)
    setTitle(value)
  }

  const handleDescriptionChange = (value: string) => {
    cache.session.set(descriptionCacheKey, value)
    setDescription(value)
  }

  const handleAttachmentsChange = (uploads: ImageUpload[]) => {
    setAttachments(uploads)
    const submissionId = cache.session.get(editingCacheKey)

    if (!submissionId) {
      return
    }

    postSubmissions.updateAttachments(account, submissionId, uploads).catch((cause) => {
      if (!(cause instanceof SubmissionStorageError)) {
        throw cause
      }

      setError({ errors: [{ message: "Attachment changes could not be saved. Keep this tab open to submit them." }] })
    })
  }
  
  const submitPost = () => {
    setError(undefined)

    setState((current) => {
      if (current.phase === "unconfirmed") {
        return { phase: "preparing", submission: current.submission }
      }

      if (current.phase === "idle" || current.phase === "countdown") {
        return { phase: "collecting" }
      }

      return current
    })
  }

  useEffect(() => {
    if (phase !== "collecting") {
      return
    }

    let mounted = true

    const collect = async () => {
      const uploads = uploader.current ? await uploader.current.readUploads() : attachments

      if (!mounted) {
        return
      }

      if (!uploads) {
        setError({ errors: [{ message: "An image could not be read. Retry or remove it before submitting." }] })
        setState({ phase: "idle" })
        return
      }

      setState({
        phase: "preparing",
        submission: {
          submissionId: newSubmissionID(),
          title,
          description,
          attachments: uploads,
        },
      })
    }

    void collect()

    return () => {
      mounted = false
    }
  }, [phase])

  useEffect(() => {
    if (state.phase !== "preparing" && state.phase !== "sending") {
      return
    }

    const { submission } = state
    const request = new AbortController()

    const send = async () => {
      try {
        const result = await sendPostSubmission(submission, request.signal)

        if (request.signal.aborted) {
          return
        }

        if (result.ok || result.status === 400) {
          if (result.ok) {
            await postSubmissions.complete(account, submission.submissionId, result.data)
          } else {
            const saved = await postSubmissions.save(account, { ...submission, rejection: result.error })

            if (request.signal.aborted) {
              return
            }

            if ("receipt" in saved) {
              finishSubmission(saved.receipt)
              return
            }

            cache.session.set(editingCacheKey, submission.submissionId)
          }
          if (request.signal.aborted) {
            return
          }
        }

        if (result.ok) {
          finishSubmission(result.data)
          return
        }

        if (result.status === 400) {
          setError(result.error)
          setState({ phase: "idle" })
          return
        }

        if (result.status && result.status < 500) {
          setError(result.error)
          setState({ phase: "unconfirmed", submission })
          return
        }
      } catch (cause) {
        if (request.signal.aborted) {
          return
        }

        if (!(cause instanceof RequestError)) {
          setError({ errors: [{ message: "The submission could not be completed. Your saved submission is retained." }] })
          setState({ phase: "unconfirmed", submission })
          if (cause instanceof SubmissionStorageError) {
            return
          }

          throw cause
        }
      }

      setState({ phase: "unconfirmed", submission })
      setError({ errors: [{ message: "The submission has not been confirmed. Retry to check and finish it." }] })
    }

    if (state.phase === "preparing") {
      cache.session.set(submissionCacheKey, submission.submissionId)
      postSubmissions.save(account, submission, cache.session.get(editingCacheKey) || undefined).then((saved) => {
        if (request.signal.aborted) {
          return
        }

        if ("receipt" in saved) {
          finishSubmission(saved.receipt)
          return
        }

        cache.session.remove(editingCacheKey)
        setState({ phase: "sending", submission })
      }).catch((cause) => {
        if (request.signal.aborted) {
          return
        }

        if (!(cause instanceof SubmissionStorageError)) {
          throw cause
        }

        setState({ phase: "unconfirmed", submission })
        setError({ errors: [{ message: "Your browser could not save this submission. Retry when storage is available." }] })
      })
    } else {
      void send()
    }

    return () => {
      request.abort()
    }
  }, [state, account])

  useEffect(() => {
    const reconnect = () => {
      if (phase === "unconfirmed") {
        submitPost()
      }

      if (phase === "load-failed") {
        setLoadVersion((version) => version + 1)
      }
    }

    window.addEventListener("online", reconnect)

    return () => window.removeEventListener("online", reconnect)
  }, [phase])

  const submit = () => {
    if (title && phase === "idle") {
      setRemainingSeconds(30)
      setState({ phase: "countdown" })
    }
  }

  const cancelSubmission = () => {
    setState({ phase: "idle" })
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

  const isSubmitDisabled = (
    title.length < titleLengthMin ||
    titleValidation.isOverMax ||
    (description.length > 0 && description.length < descriptionLengthMin) ||
    descValidation.isOverMax ||
    isPostingDisabled
  )

  const progressPercentage = ((30 - remainingSeconds) / 30) * 100

  return (
    <>
      <SavedPostRecovery account={account} submissions={otherSubmissions} />
      <Form error={error}>
        {phase === "load-failed" && (
          <Button onClick={() => setLoadVersion((version) => version + 1)}>Reload saved submission</Button>
        )}
        {phase === "unconfirmed" && isPostingDisabled && (
          <Button onClick={submitPost}>Retry submission</Button>
        )}
        {isPostingDisabled && (
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
                disabled={fider.isReadOnly || isPostingDisabled || isFrozen}
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
                    disabled={isPostingDisabled || isFrozen}
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
                {phase !== "loading" && (
                  <MultiImageUploader
                    ref={uploader}
                    field="attachments"
                    maxUploads={maxImagesPerPost}
                    initialUploads={attachments}
                    disabled={isFrozen}
                    onChange={handleAttachmentsChange}
                  />
                )}
                
                {isPendingSubmission ? (
                  <div className="flex justify-between items-center">
                    <div className="flex gap-2">
                      <Button type="button" variant="secondary" onClick={submitPost}>
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
                    <Button
                      type="submit"
                      variant="primary"
                      loading={isSending}
                      disabled={phase !== "unconfirmed" && (isSubmitDisabled || isFrozen)}
                      onClick={phase === "unconfirmed" ? submitPost : submit}
                    >
                      {phase === "unconfirmed" ? "Retry submission" : <Trans id="action.submit">Submit</Trans>}
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
