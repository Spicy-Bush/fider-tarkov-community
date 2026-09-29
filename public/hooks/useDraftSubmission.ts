import { useEffect, useRef, useState } from "react"
import { ImageUpload } from "@fider/models"
import { Failure } from "@fider/services"
import { AccountDraft, DraftPayload, DraftReceipt } from "@fider/services/browserDrafts"
import { DraftImage, prepareDraftImages } from "@fider/services/draftImages"
import { RetryResult } from "@fider/services/retryRequest"
import { useAccountDraft } from "./useAccountDraft"

interface AttachmentDraft extends DraftPayload {
  attachments: DraftImage[]
}

type SealedDraft<T> = { payload: T & { submissionId: string }; draft: DraftReceipt }
type RequestPayload<T> = Omit<T, "attachments"> & { submissionId: string; attachments: ImageUpload[] }
type SubmissionState<T> =
  | { phase: "idle" | "sealing" | "complete" }
  | { phase: "preparing" | "sending" | "unconfirmed"; submission: SealedDraft<T> }

function isActive<T>(state: SubmissionState<T>): boolean {
  return state.phase === "sealing" || state.phase === "preparing" || state.phase === "sending"
}

interface SubmissionOptions<T extends AttachmentDraft, R> {
  editor: ReturnType<typeof useAccountDraft<T>>
  send: (payload: RequestPayload<T>, signal: AbortSignal) => Promise<RetryResult<R>>
  onSaved: (result: R, completion: { firstCompletion: boolean; mounted: boolean }) => void
  onUnauthorized: () => void
}

export function useDraftSubmission<T extends AttachmentDraft, R>(options: SubmissionOptions<T, R>) {
  const [state, setState] = useState<SubmissionState<T>>({ phase: "idle" })
  const [error, setError] = useState<Failure>()
  const current = useRef(state)
  const mounted = useRef(false)
  const request = useRef<AbortController>()

  const publish = (next: SubmissionState<T>) => {
    current.current = next
    if (mounted.current) setState(next)
  }

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      request.current?.abort()
    }
  }, [])

  const select = (saved: AccountDraft<T>) => {
    if (isActive(current.current)) return

    const submission = {
      payload: { ...saved.payload, submissionId: saved.id },
      draft: { id: saved.id, revision: saved.revision },
    }
    publish({ phase: "unconfirmed", submission })
    setError(undefined)
  }

  const reopen = async (submission: SealedDraft<T>) => {
    try {
      await options.editor.reopen(submission.payload, submission.draft)
    } catch (cause) {
      console.error("Could not save the rejected draft.", cause)
    }
    publish({ phase: "idle" })
  }

  const submit = async (saved?: AccountDraft<T>) => {
    if (isActive(current.current)) return
    if (saved) select(saved)

    const previouslyUnconfirmed = current.current.phase === "unconfirmed"
    let submission = "submission" in current.current ? current.current.submission : undefined
    publish(submission ? { phase: "preparing", submission } : { phase: "sealing" })
    setError(undefined)
    let payload: RequestPayload<T>

    try {
      submission ??= await options.editor.seal()
      publish({ phase: "preparing", submission })
      const { attachments, ...fields } = submission.payload
      payload = { ...fields, attachments: await prepareDraftImages(attachments) }
    } catch (cause) {
      if (submission && previouslyUnconfirmed) {
        publish({ phase: "unconfirmed", submission })
      } else if (submission) {
        await reopen(submission)
      } else {
        publish({ phase: "idle" })
      }

      if (mounted.current) {
        setError({ errors: [{ message: "An attachment could not be read. Check your images and retry." }], cause })
      }
      return
    }
    if (!mounted.current) return

    publish({ phase: "sending", submission })
    const controller = new AbortController()
    request.current = controller
    let result: RetryResult<R>
    try {
      result = await options.send(payload, controller.signal)
    } catch (cause) {
      publish({ phase: "unconfirmed", submission })
      if (mounted.current) {
        setError({ errors: [{ message: "The submission has not been confirmed. Retry to check and finish it." }], cause })
      }
      return
    }

    if (!result.ok) {
      if (previouslyUnconfirmed || result.unconfirmed) {
        publish({ phase: "unconfirmed", submission })
      } else {
        await reopen(submission)
      }

      if (mounted.current) {
        setError(result.error)
        if (result.status === 401) options.onUnauthorized()
      }
      return
    }

    let firstCompletion = false
    try {
      firstCompletion = await options.editor.complete(submission.draft)
    } catch (cause) {
      console.error("Could not clear the completed draft.", cause)
    }

    options.editor.reset()
    publish({ phase: "complete" })
    options.onSaved(result.data, { firstCompletion, mounted: mounted.current })
  }

  const startNew = () => {
    options.editor.reset()
    publish({ phase: "idle" })
    setError(undefined)
  }

  return {
    state,
    busy: isActive(state),
    error,
    submit,
    select,
    startNew,
  }
}
