import React, { useCallback, useEffect, useRef, useState } from "react"
import { UserAvatarType } from "@fider/models"
import { http } from "@fider/services/http"

interface ProfileChange {
  field: "name" | "avatar"
  state: "pending" | "running" | "complete" | "failed" | "rejected"
  published?: boolean
  previewURL?: string
  value: string
  revision: number
}

interface ProfileStatus {
  changes: ProfileChange[]
  name: string
  avatarType: UserAvatarType
  avatarURL: string
}

interface Props {
  enabled: boolean
  onNameChanged: (name: string) => void
  onAvatarChanged: (url: string, avatarType: UserAvatarType) => void
}

export function useProfileReview({ enabled, onNameChanged, onAvatarChanged }: Props) {
  const [changes, setChanges] = useState<ProfileChange[]>([])
  const [unavailable, setUnavailable] = useState(false)
  const [settling, setSettling] = useState(false)
  const cancelRefresh = useRef<() => void>(() => {})
  const callbacks = useRef({ onNameChanged, onAvatarChanged })
  callbacks.current = { onNameChanged, onAvatarChanged }

  const pause = useCallback(() => {
    cancelRefresh.current()
  }, [])

  const refresh = useCallback(
    (waitFor?: ProfileChange["field"]): Promise<void> => {
      pause()

      if (!enabled) {
        return Promise.resolve()
      }

      return new Promise((resolve) => {
        let active = true
        let waiting = !!waitFor
        let pollTimer: number | undefined
        let waitTimer: number | undefined
        setSettling(waiting)

        const finishWaiting = () => {
          waiting = false
          window.clearTimeout(waitTimer)
          setSettling(false)
          resolve()
        }

        cancelRefresh.current = () => {
          active = false
          window.clearTimeout(pollTimer)
          window.clearTimeout(waitTimer)
          resolve()
        }

        if (waiting) {
          waitTimer = window.setTimeout(finishWaiting, 2000)
        }

        const poll = async () => {
          try {
            const result = await http.get<ProfileStatus>("/api/user/moderation")

            if (!active) {
              return
            }

            if (!result.ok) {
              throw new Error("Status unavailable")
            }

            setUnavailable(false)
            setChanges(result.data.changes)
            callbacks.current.onNameChanged(result.data.name)
            callbacks.current.onAvatarChanged(result.data.avatarURL, result.data.avatarType)

            const unfinished = result.data.changes.filter((change) => ["pending", "running", "failed"].includes(change.state))
            const awaitingPublication = unfinished.some((change) => change.field === waitFor && !change.published && change.state !== "failed")

            if (!awaitingPublication) {
              finishWaiting()
            }

            if (unfinished.length > 0) {
              pollTimer = window.setTimeout(poll, waiting ? 500 : 5000)
            }
          } catch {
            if (active) {
              setUnavailable(true)
              finishWaiting()
              pollTimer = window.setTimeout(poll, 10000)
            }
          }
        }

        void poll()
      })
    },
    [enabled, pause]
  )

  useEffect(() => {
    void refresh()

    return pause
  }, [refresh, pause])

  return { changes, unavailable, settling, refresh, pause }
}

export function ProfileReviewStatus({ changes, unavailable, settling }: ReturnType<typeof useProfileReview>) {
  return (
    <div aria-live="polite" className="text-sm text-muted space-y-1">
      {unavailable && <p>Check status is temporarily unavailable. Retrying automatically.</p>}
      {changes
        .filter((change) => change.state !== "complete" && !(settling && ["pending", "running"].includes(change.state)))
        .map((change) => {
          let message = " is saved and awaiting a check. It will appear automatically when approved."

          if (change.state === "rejected") {
            message = " wasn’t accepted. Please choose another."
          } else if (change.published) {
            message = " is visible. Its check is delayed and will retry automatically."
          } else if (change.state === "failed") {
            message = " is saved, but its check needs staff attention. Your current profile is unchanged."
          }

          return (
            <p key={change.field}>
              {change.previewURL && (
                <img src={change.previewURL} alt="Saved avatar awaiting approval" className="w-12 h-12 rounded-full object-cover inline-block mr-2" />
              )}
              {change.field === "name" ? `Name “${change.value}”` : "Avatar"}
              {message}
            </p>
          )
        })}
    </div>
  )
}