import React, { useState, useEffect, useRef } from "react"
import { Button, Input, Form, TextArea, MultiImageUploader, SignInModal } from "@fider/components"
import { PreviewPostModal } from "./PreviewPostModal"
import { analytics } from "@fider/services/analytics"
import { sendPostSubmission } from "@fider/services/postSubmission"
import { AccountDraft } from "@fider/services/browserDrafts"
import { DraftImage } from "@fider/services/draftImages"
import { useFider } from "@fider/hooks"
import { useAccountDraft } from "@fider/hooks/useAccountDraft"
import { useDraftSubmission } from "@fider/hooks/useDraftSubmission"
import { DraftStatus } from "@fider/components/common/DraftStatus"
import { DraftPicker } from "@fider/components/common/DraftPicker"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface PostInputProps {
  placeholder: string
  onTitleChanged: (title: string) => void
}

interface PostContent {
  title: string
  description: string
  attachments: DraftImage[]
}

const submissionCacheKey = "PostInput-Submission"

function rememberSubmission(id: string | null) {
  try {
    if (id) window.sessionStorage.setItem(submissionCacheKey, id)
    else window.sessionStorage.removeItem(submissionCacheKey)
  } catch (cause) {
    console.error("Could not remember the selected submission.", cause)
  }
}

export const PostInput = (props: PostInputProps) => {
  const fider = useFider()
  const account = `${fider.session.tenant.id}:${fider.session.isAuthenticated ? fider.session.user.id : "anonymous"}`

  return <PostDraft key={account} {...props} account={account} />
}

const PostDraft = (props: PostInputProps & { account: string }) => {
  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const draft = useAccountDraft<PostContent>({
    kind: "post",
    scope: "post:new",
    initial: { title: "", description: "", attachments: [] },
  })
  const [countingDown, setCountingDown] = useState(false)
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [isPreviewModalOpen, setIsPreviewModalOpen] = useState(false)
  const [remainingSeconds, setRemainingSeconds] = useState(30)
  const [selectedID] = useState(() => {
    try {
      return new URLSearchParams(window.location.search).get("submission") || window.sessionStorage.getItem(submissionCacheKey)
    } catch {
      return null
    }
  })
  const restored = useRef(false)
  const edited = useRef(false)
  const submission = useDraftSubmission({
    editor: draft,
    send: (payload, signal) => {
      rememberSubmission(payload.submissionId)
      return sendPostSubmission(payload, signal)
    },
    onUnauthorized: () => setIsSignInModalOpen(true),
    onSaved: (post, { firstCompletion, mounted }) => {
      if (firstCompletion) analytics.event("post", "create")
      rememberSubmission(null)
      if (mounted) location.href = `/posts/${post.number}/${post.slug}`
    },
  })
  const { state, error } = submission
  const phase = state.phase === "idle" && countingDown ? "countdown" : state.phase
  const { title, description, attachments } = "submission" in state ? state.submission.payload : draft.value
  const isPendingSubmission = phase === "countdown"
  const isSending = submission.busy || phase === "complete"
  const isFrozen = phase !== "idle" && phase !== "countdown"
  const isPostingDisabled = fider.session.isAuthenticated && !fider.session.permissions.createPosts

  const settings = fider.session.tenant.generalSettings || {
    titleLengthMin: 15,
    titleLengthMax: 100,
    descriptionLengthMin: 150,
    descriptionLengthMax: 1000,
    maxImagesPerPost: 3,
  }
  const { titleLengthMin, titleLengthMax, descriptionLengthMin, descriptionLengthMax, maxImagesPerPost } = settings

  useEffect(() => { props.onTitleChanged(title) }, [title])

  const resume = (saved: AccountDraft<PostContent>) => submission.submit(saved)

  useEffect(() => {
    if (!draft.loaded || restored.current) return

    restored.current = true
    const saved = draft.pending.find(item => item.id === selectedID)
    if (!edited.current && saved) void resume(saved)
  }, [draft.loaded, draft.pending])

  useEffect(() => {
    if (state.phase === "idle" && error) rememberSubmission(null)
  }, [state.phase, error])

  const submitPost = () => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }

    setCountingDown(false)
    void submission.submit()
  }

  useEffect(() => {
    if (!isPendingSubmission) return
    if (remainingSeconds === 0) {
      submitPost()
      return
    }

    const timer = setTimeout(() => setRemainingSeconds(remainingSeconds - 1), 1000)
    return () => clearTimeout(timer)
  }, [phase, remainingSeconds])

  useEffect(() => {
    const reconnect = () => {
      if (phase === "unconfirmed") submitPost()
    }
    window.addEventListener("online", reconnect)
    return () => window.removeEventListener("online", reconnect)
  }, [phase])

  const submit = () => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }
    if (title && phase === "idle") {
      setRemainingSeconds(30)
      setCountingDown(true)
    }
  }

  const cancelSubmission = () => {
    setCountingDown(false)
    setRemainingSeconds(30)
  }

  const change = (value: Partial<PostContent>) => {
    edited.current = true
    draft.change(value)
  }
  const handleTitleChange = (value: string) => change({ title: value })
  const handleDescriptionChange = (value: string) => change({ description: value })
  const handleAttachmentsChange = (value: DraftImage[]) => change({ attachments: value })
  const showPreview = () => setIsPreviewModalOpen(true)

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
    descValidation.isOverMax
  )

  const progressPercentage = ((30 - remainingSeconds) / 30) * 100

  const selectedSubmission = "submission" in state ? state.submission.draft.id : undefined
  const pendingChoices = draft.pending.filter(item => item.id !== selectedSubmission)

  return (
    <>
      {pendingChoices.length > 0 && <div className="my-2">
        <DraftPicker<PostContent>
          drafts={pendingChoices}
          label="Pending submissions"
          action="Continue pending submission"
          disabled={isFrozen}
          onSelect={resume}
        />
      </div>}
      <SignInModal isOpen={isSignInModalOpen} onClose={() => setIsSignInModalOpen(false)} />
      <Form error={error}>
        <DraftStatus {...draft} />
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
        <div className="relative">
          <Input
            field="title"
            disabled={isFrozen}
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
                disabled={isFrozen}
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
            <MultiImageUploader
              field="attachments"
              maxUploads={maxImagesPerPost}
              value={attachments}
              disabled={isFrozen}
              onChange={handleAttachmentsChange}
            />

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
