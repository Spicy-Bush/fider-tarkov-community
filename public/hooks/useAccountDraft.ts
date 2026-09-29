import { useEffect, useRef, useState } from "react"
import { useFider } from "./use-fider"
import { AccountDraft, accountDrafts, DraftKind, DraftPayload, DraftReceipt, DraftUpdate } from "@fider/services/browserDrafts"
import { newSubmissionID } from "@fider/services/postSubmission"

export interface DraftOptions<T extends DraftPayload> {
  kind: DraftKind
  scope: string
  initial: T
  importLocal?: boolean
}

interface EditorState<T extends DraftPayload> {
  value: T
  status: "idle" | "loading" | "saved" | "saving" | "error"
  loaded: boolean
  error?: string
  restored: number
  saved: AccountDraft<T>[]
}

export function useAccountDraft<T extends DraftPayload>(options: DraftOptions<T>) {
  const fider = useFider()
  const account = `${fider.session.tenant.id}:${fider.session.isAuthenticated ? fider.session.user.id : "anonymous"}`
  const [state, setState] = useState<EditorState<T>>({
    value: options.initial,
    status: "loading",
    loaded: false,
    restored: 0,
    saved: [],
  })
  const current = useRef(state)
  const mounted = useRef(false)
  const identity = useRef<DraftReceipt>()
  const dirty = useRef(false)
  const initializing = useRef<Promise<void>>()
  const writing = useRef<Promise<T>>()
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const replacement = useRef<DraftReceipt>()
  const sealing = useRef<Promise<{ payload: T & { submissionId: string }; draft: DraftReceipt }>>()
  const refreshing = useRef(new Map<string, object>())

  const getIdentity = () => {
    if (!identity.current) identity.current = { id: newSubmissionID(), revision: 0 }
    return identity.current
  }

  const publish = (change: Partial<EditorState<T>>) => {
    current.current = { ...current.current, ...change }
    if (mounted.current) setState(current.current)
  }

  const failed = () => publish({
    status: "error",
    error: "Your browser could not save this draft. Keep this tab open and retry.",
  })

  const remember = (id: string, saved: AccountDraft<T> | null) => {
    const records = current.current.saved.filter(item => item.id !== id)
    if (saved?.kind === options.kind && saved.scope === options.scope) records.push(saved)
    records.sort((left, right) => right.updatedAt - left.updatedAt)
    publish({ saved: records })
  }

  const adopt = (saved: AccountDraft<T>) => {
    identity.current = { id: saved.id, revision: saved.revision }
    dirty.current = false
    publish({
      value: { ...options.initial, ...saved.payload },
      restored: current.current.restored + 1,
      status: "saved",
      error: undefined,
    })
  }

  const initialize = (): Promise<void> => {
    if (initializing.current) return initializing.current

    initializing.current = (async () => {
      try {
        if (fider.session.isAuthenticated) {
          await accountDrafts.adoptAnonymous(account, options.kind, options.scope)
        }

        const saved = await accountDrafts.list(account, options.scope)
        const matching = saved.filter(item => item.kind === options.kind && item.scope === options.scope) as AccountDraft<T>[]
        const editable = matching.filter(item => item.phase === "editable")
        if (!current.current.loaded && !dirty.current && editable.length > 0) adopt(editable[0])
        if (!dirty.current && editable.length === 0 && options.importLocal) {
          dirty.current = true
          accountDrafts.markUnsaved(getIdentity().id, true)
        }

        publish({
          saved: matching,
          status: current.current.status === "loading" || current.current.status === "error" ? "idle" : current.current.status,
          error: undefined,
        })
      } catch (cause) {
        initializing.current = undefined
        console.error("Could not load browser drafts.", cause)
        failed()
      } finally {
        publish({ loaded: true })
      }
    })()
    return initializing.current
  }

  const flush = (): Promise<T> => {
    clearTimeout(timer.current)
    if (writing.current) return writing.current

    writing.current = (async () => {
      await initialize()
      if (!dirty.current) return current.current.value

      publish({ status: "saving", error: undefined })
      while (dirty.current) {
        const value = current.current.value
        const saved = await accountDrafts.save(account, {
          ...getIdentity(),
          kind: options.kind,
          scope: options.scope,
          payload: value,
        }, replacement.current)

        if (!saved.accepted) {
          accountDrafts.markUnsaved(getIdentity().id, false)
          identity.current = { id: newSubmissionID(), revision: 0 }
          accountDrafts.markUnsaved(identity.current.id, true)
          if (saved.draft) remember(saved.draft.id, saved.draft)
          continue
        }

        identity.current = { id: saved.draft.id, revision: saved.draft.revision }
        remember(saved.draft.id, saved.draft)
        replacement.current = undefined
        dirty.current = value !== current.current.value
      }

      accountDrafts.markUnsaved(getIdentity().id, false)
      publish({ status: "saved", error: undefined })
      return current.current.value
    })().catch(cause => {
      failed()
      throw cause
    }).finally(() => {
      writing.current = undefined
    })
    return writing.current
  }

  const change = (update: Partial<T> | ((value: T) => T)) => {
    sealing.current = undefined
    const value = typeof update === "function" ? update(current.current.value) : { ...current.current.value, ...update }
    dirty.current = true
    accountDrafts.markUnsaved(getIdentity().id, true)
    publish({ value, status: "saving", error: undefined })
    clearTimeout(timer.current)
    timer.current = setTimeout(() => { void flush().catch(() => {}) }, 300)
  }

  useEffect(() => {
    mounted.current = true
    void flush().catch(() => {})
    const retry = () => { void flush().catch(() => {}) }
    const refresh = (event: Event) => {
      const update = (event as CustomEvent<DraftUpdate>).detail
      if (update.account !== account) return

      if ("expired" in update) {
        refreshing.current.clear()
        publish({ saved: [], status: current.current.status === "saved" ? "idle" : current.current.status })
        return
      }

      if (update.scope && update.scope !== options.scope) return
      if (update.id === identity.current?.id && writing.current) return

      const read = {}
      refreshing.current.set(update.id, read)
      void initialize().then(() => accountDrafts.resume<T>(account, update.id)).then(saved => {
        if (!mounted.current || refreshing.current.get(update.id) !== read) return

        remember(update.id, saved)
      }).catch(() => {
        if (!mounted.current || refreshing.current.get(update.id) !== read) return

        initializing.current = undefined
        publish({ status: "error", error: "Saved drafts could not be refreshed. Keep this tab open and retry." })
      }).finally(() => {
        if (refreshing.current.get(update.id) === read) refreshing.current.delete(update.id)
      })
    }
    window.addEventListener("focus", retry)
    window.addEventListener("account-drafts-changed", refresh)

    return () => {
      mounted.current = false
      clearTimeout(timer.current)
      window.removeEventListener("focus", retry)
      window.removeEventListener("account-drafts-changed", refresh)
      refreshing.current.clear()
      if (dirty.current) void flush().catch(() => {})
    }
  }, [])

  const seal = () => {
    if (sealing.current) return sealing.current

    sealing.current = (async () => {
      let value: T
      let receipt: DraftReceipt
      try {
        if (getIdentity().revision === 0) dirty.current = true
        for (;;) {
          await flush()
          const saved = await accountDrafts.seal<T>(account, getIdentity())
          if (saved.accepted) {
            value = saved.draft.payload
            receipt = { id: saved.draft.id, revision: saved.draft.revision }
            remember(receipt.id, saved.draft)
            accountDrafts.markUnsaved(receipt.id, false)
            break
          }

          if (saved.draft) remember(saved.draft.id, saved.draft)
          accountDrafts.markUnsaved(getIdentity().id, false)
          identity.current = { id: newSubmissionID(), revision: 0 }
          dirty.current = true
        }
      } catch {
        accountDrafts.markUnsaved(getIdentity().id, false)
        receipt = { id: newSubmissionID(), revision: 0 }
        value = current.current.value
        accountDrafts.markUnsaved(receipt.id, true)
        failed()
      }

      identity.current = undefined
      dirty.current = false
      return { payload: { ...value, submissionId: receipt.id }, draft: receipt }
    })()
    return sealing.current
  }

  const reset = () => {
    if (identity.current) accountDrafts.markUnsaved(identity.current.id, false)
    identity.current = undefined
    dirty.current = false
    sealing.current = undefined
    publish({ value: options.initial, restored: current.current.restored + 1, status: "idle", error: undefined })
  }

  const resume = async (saved: AccountDraft<T>) => {
    try {
      await flush()
      const previous = current.current.value
      const selected = await accountDrafts.resume<T>(account, saved.id)
      if (previous !== current.current.value) return

      if (!selected) {
        remember(saved.id, null)
        publish({ status: "error", error: "This draft has expired or was removed. Your current text is retained." })
      } else if (selected.phase === "pending") {
        remember(selected.id, selected)
      } else {
        remember(selected.id, selected)
        adopt(selected)
      }
    } catch (cause) {
      publish({ status: "error", error: "This draft could not be opened. Your current work is retained; retry the selection." })
      throw cause
    }
  }

  const reopen = async (value: T, receipt?: DraftReceipt) => {
    const previous = current.current.value
    const { submissionId: _, ...editable } = value as T & { submissionId?: string }
    await writing.current?.catch(() => {})
    await initialize()

    identity.current = { id: newSubmissionID(), revision: 0 }
    replacement.current = receipt
    change(previous === current.current.value ? editable as T : current.current.value)
    return flush()
  }

  const complete = async (receipt: DraftReceipt): Promise<boolean> => {
    accountDrafts.markUnsaved(receipt.id, false)
    try {
      return await accountDrafts.remove(account, receipt)
    } finally {
      remember(receipt.id, null)
    }
  }

  const { saved, ...editor } = state
  const alternatives = saved.filter(item => item.phase === "editable" && item.id !== identity.current?.id)
  const pending = saved.filter(item => item.phase === "pending")
  return { ...editor, alternatives, pending, change, flush, seal, resume, reset, reopen, complete, receipt: () => ({ ...getIdentity() }) }
}
