import { useState, useSyncExternalStore } from "react"
import { Failure, RequestError, Result, requestOutcome } from "@fider/services/http"
import * as notify from "@fider/services/notify"
import { newSubmissionID } from "@fider/services/postSubmission"

type SaveState<T> =
  | { phase: "idle"; error?: Failure }
  | { phase: "saving"; value: T }
  | { phase: "uncertain"; value: T; identity: string; error: Failure }

export function useSponsorSave<T, R>(
  send: (value: T, identity: string) => Promise<Result<R>>,
  saved: (value: R) => void,
  issueIdentity?: () => Promise<Result<string>>
) {
  const [owner] = useState(() => {
    let state: SaveState<T> = { phase: "idle" }
    const listeners = new Set<() => void>()

    return {
      snapshot: () => state,
      subscribe: (listener: () => void) => {
        listeners.add(listener)
        return () => { listeners.delete(listener) }
      },
      update: (next: SaveState<T>) => {
        state = next
        for (const listener of listeners) listener()
      },
    }
  })
  const state = useSyncExternalStore(owner.subscribe, owner.snapshot, owner.snapshot)

  const execute = async (value: T, identity?: string) => {
    owner.update({ phase: "saving", value })

    const failed = (error: Failure, uncertain: boolean) => {
      owner.update(uncertain && identity
        ? { phase: "uncertain", value, identity, error }
        : { phase: "idle", error })
      notify.error(error.errors?.map(item => item.message).join(" ") || "The save could not be confirmed. Please retry.")
    }

    try {
      if (!identity) {
        const issued = issueIdentity ? await issueIdentity() : { ok: true as const, data: newSubmissionID() }
        if (!issued.ok) {
          failed(issued.error, false)
          return
        }

        identity = issued.data
      }

      const result = await send(value, identity)
      if (!result.ok) {
        failed(result.error, requestOutcome(result) === "uncertain")
        return
      }

      owner.update({ phase: "idle" })
      saved(result.data)
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        owner.update({ phase: "idle" })
        throw cause
      }

      failed({ errors: [{ message: "The save could not be confirmed. Retry to check the same save." }], cause }, true)
    }
  }

  return {
    busy: state.phase === "saving",
    uncertain: state.phase === "uncertain",
    disabled: state.phase !== "idle",
    pending: state.phase === "idle" ? undefined : state.value,
    error: state.phase === "saving" ? undefined : state.error,
    submit: (value: T) => {
      if (owner.snapshot().phase === "idle") return execute(value)
    },
    retry: () => {
      const current = owner.snapshot()
      if (current.phase === "uncertain") return execute(current.value, current.identity)
    },
  }
}
