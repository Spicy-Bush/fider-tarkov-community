import { useEffect, useRef, useState } from "react"
import type * as Y from "yjs"
import type { Awareness } from "y-protocols/awareness"
import { useFider } from "./use-fider"
import { Page, PageDraft } from "@fider/models"
import { Failure, http, RequestError } from "@fider/services/http"
import { AccountDraft, accountDrafts, DraftReceipt } from "@fider/services/browserDrafts"
import { DraftImage, prepareDraftImages } from "@fider/services/draftImages"
import { newSubmissionID } from "@fider/services/postSubmission"
import { changePageDocument, decodePageState, encodePageState, PageWorkingCopy, readPageDocument, pageScheduleInput } from "@fider/services/pageCollaboration"

interface RecoveryPayload {
  state: Uint8Array
  bannerImage?: DraftImage | null
}

interface SharedPage {
  pageId: number
  state: string
  stateVector: string
  legacyDrafts: PageDraft[]
}

interface SyncResult {
  update: string
  stateVector: string
}

interface Collaboration {
  document: Y.Doc
  awareness: Awareness
  pageId: number
}

export function initialPageWorkingCopy(page?: Page): PageWorkingCopy {
  return {
    title: page?.title || "",
    slug: page?.slug || "",
    content: page?.content || "",
    excerpt: page?.excerpt || "",
    metaDescription: page?.metaDescription || "",
    bannerImage: page?.bannerImageBKey ? { kind: "stored", bkey: page.bannerImageBKey } : null,
    status: page?.status || "draft",
    visibility: page?.visibility || "public",
    parentPageId: page?.parentPageId || null,
    allowComments: page?.allowComments || false,
    allowCommentImages: page?.allowCommentImages || false,
    allowReactions: page?.allowReactions ?? true,
    showToc: page?.showToc || false,
    scheduledFor: pageScheduleInput(page?.scheduledFor || ""),
    topics: page?.topics?.map(topic => topic.id) || [],
    tags: page?.tags?.map(tag => tag.id) || [],
    authors: page?.authors?.map(author => author.id) || [],
    allowedRoles: page?.allowedRoles || [],
  }
}

export function usePageCollaboration(page?: Page) {
  const fider = useFider()
  const account = `${fider.session.tenant.id}:${fider.session.user?.id}`
  const [value, setValue] = useState(() => initialPageWorkingCopy(page))
  const [session, setSession] = useState<Collaboration>()
  const [error, setError] = useState<Failure>()
  const [recoveryError, setRecoveryError] = useState<string>()
  const [alternatives, setAlternatives] = useState<AccountDraft<Partial<PageWorkingCopy>>[]>([])
  const actions = useRef<{
    change: (change: Partial<PageWorkingCopy> | ((current: PageWorkingCopy) => Partial<PageWorkingCopy>)) => void
    flush: () => Promise<void>
    restore: (draft: AccountDraft<Partial<PageWorkingCopy>>) => Promise<void>
    retry: () => Promise<void>
  }>()
  const restart = useRef(0)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let Y: typeof import("yjs")
    let awarenessProtocol: typeof import("y-protocols/awareness")
    let closed = false
    let connection: EventSource | undefined
    let collaboration: Collaboration | undefined
    let timer: ReturnType<typeof setTimeout> | undefined
    let cursorTimer: ReturnType<typeof setTimeout> | undefined
    let retryTimer: ReturnType<typeof setTimeout> | undefined
    let syncing: Promise<void> | undefined
    let storing: Promise<void> | undefined
    let storedVersion = 0
    let serverVector: Uint8Array = new Uint8Array([0])
    const requests = new AbortController()
    let pendingBanner: DraftImage | null | undefined
    let localVersion = 0
    let acceptedVersion = 0
    let receipt: DraftReceipt = { id: newSubmissionID(), revision: 0 }
    let recovered: AccountDraft<RecoveryPayload>[] = []
    let recoveryReadFailed = false
    let retained: AccountDraft<Partial<PageWorkingCopy>>[] = []
    const origin = {}
    const remote = {}

    const fail = (cause: unknown) => {
      if (closed) {
        return
      }

      setError(cause && typeof cause === "object" && "errors" in cause
        ? cause as Failure
        : { errors: [{ message: "Your changes are still in this editor. Reconnect and retry to save them." }], cause })
    }

    const current = () => {
      const values = readPageDocument(collaboration!.document)
      return pendingBanner === undefined ? values : { ...values, bannerImage: pendingBanner }
    }

    const loadRecovery = async () => {
      try {
        const pageId = collaboration?.pageId || page?.id
        const [saved, old] = await Promise.all([
          pageId ? accountDrafts.list(account, `page-collaboration:${pageId}`) : Promise.resolve([]),
          accountDrafts.list(account, `page:${page?.id || "new"}`),
        ])
        recovered = saved as AccountDraft<RecoveryPayload>[]
        recoveryReadFailed = false
        setAlternatives([...(old as AccountDraft<Partial<PageWorkingCopy>>[]), ...retained])
        setRecoveryError(undefined)
      } catch (cause) {
        recoveryReadFailed = true
        setRecoveryError("Saved browser work could not be loaded. Retry before leaving this tab.")
        console.error("Could not load Page recovery data.", cause)
      }
    }

    const applyRecovery = () => {
      for (const saved of recovered) {
        Y.applyUpdate(collaboration!.document, saved.payload.state, remote)
        if (saved.payload.bannerImage !== undefined && pendingBanner === undefined) {
          pendingBanner = saved.payload.bannerImage
        }
      }

      if (recovered.length > 0) {
        localVersion++
        accountDrafts.markUnsaved(receipt.id, true)
      }
    }

    const persist = () => {
      if (storing) {
        return
      }

      storing = (async () => {
        while (collaboration && storedVersion < localVersion) {
          const version = localVersion
          const result = await accountDrafts.save(account, {
            ...receipt,
            kind: "page",
            scope: `page-collaboration:${collaboration.pageId}`,
            payload: {
              state: Y.encodeStateAsUpdate(collaboration.document),
              ...(pendingBanner !== undefined && { bannerImage: pendingBanner }),
            },
          })
          if (!result.accepted) {
            accountDrafts.markUnsaved(receipt.id, false)
            receipt = { id: newSubmissionID(), revision: 0 }
            accountDrafts.markUnsaved(receipt.id, localVersion !== acceptedVersion)
            continue
          }

          receipt = { id: result.draft.id, revision: result.draft.revision }
          storedVersion = version
          if (!closed && !recoveryReadFailed) {
            setRecoveryError(undefined)
          }
        }
      })().catch(cause => {
        if (!closed) {
          setRecoveryError("Your browser could not save a recovery copy. Keep this tab open until it reconnects.")
        }
        console.error("Could not store unsent Page changes.", cause)
      }).finally(() => {
        storing = undefined
      })
    }

    const synchronize = (): Promise<void> => {
      clearTimeout(timer)
      clearTimeout(retryTimer)
      if (syncing) {
        return syncing
      }
      if (!collaboration) {
        return Promise.reject(new Error("The shared Page is not loaded."))
      }

      syncing = (async () => {
        do {
          if (pendingBanner?.kind === "missing") {
            throw new Error(`Select ${pendingBanner.fileName} again before saving.`)
          }

          if (pendingBanner?.kind === "local") {
            const image = pendingBanner
            const [upload] = await prepareDraftImages([image])
            const uploaded = await http.post<{ bkey: string }>(`/api/pages/${collaboration!.pageId}/draft/banner`, {
              submissionId: image.fileId,
              image: upload,
            }, { notifyOnError: false, signal: requests.signal })
            if (!uploaded.ok) {
              throw { ...uploaded.error, status: uploaded.status }
            }

            if (pendingBanner === image) {
              pendingBanner = undefined
              changePageDocument(collaboration!.document, { bannerImage: { kind: "stored", bkey: uploaded.data.bkey } }, origin)
            }
          }

          const sentVersion = localVersion
          const document = collaboration!.document
          const response = await http.post<SyncResult>(`/api/pages/${collaboration!.pageId}/draft`, {
            update: encodePageState(Y.encodeStateAsUpdate(document, serverVector)),
            stateVector: encodePageState(Y.encodeStateVector(document)),
          }, { notifyOnError: false, signal: requests.signal })
          if (!response.ok) {
            throw { ...response.error, status: response.status }
          }

          Y.applyUpdate(document, decodePageState(response.data.update), remote)
          serverVector = decodePageState(response.data.stateVector)
          acceptedVersion = sentVersion
          accountDrafts.markUnsaved(receipt.id, localVersion !== acceptedVersion || pendingBanner !== undefined)
          if (!closed) {
            setError(undefined)
            if (!recoveryReadFailed && localVersion === acceptedVersion && pendingBanner === undefined) {
              setRecoveryError(undefined)
            }
          }
        } while (!closed && (localVersion !== acceptedVersion || pendingBanner?.kind === "local"))

        if (closed) {
          return
        }
        await Promise.all(recovered.map(item => accountDrafts.remove(account, item))).catch(cause => {
          console.error("Could not remove acknowledged Page recovery data.", cause)
        })
        recovered = []
      })().catch(cause => {
        fail(cause)
        throw cause
      }).finally(() => {
        syncing = undefined
      })

      return syncing
    }

    const schedule = () => {
      clearTimeout(timer)
      timer = setTimeout(() => synchronize().catch((cause: unknown) => {
        const status = cause && typeof cause === "object" && "status" in cause ? cause.status : undefined
        if (!(cause instanceof RequestError) && !(typeof status === "number" && status >= 500)) {
          return
        }

        clearTimeout(retryTimer)
        retryTimer = setTimeout(() => {
          if (!closed) {
            schedule()
          }
        }, 3000)
      }), 250)
    }

    const initialize = async () => {
      const modules = await Promise.all([import("yjs"), import("y-protocols/awareness")])
      Y = modules[0]
      awarenessProtocol = modules[1]
      if (closed) {
        return
      }

      const creationKey = `page-creation:${account}`
      let submissionId = newSubmissionID()
      if (!page) {
        try {
          submissionId = sessionStorage.getItem(creationKey) || submissionId
          sessionStorage.setItem(creationKey, submissionId)
        } catch (cause) {
          setRecoveryError("Your browser could not retain this new Page identity. Keep the tab open until it connects.")
          console.error("Could not retain the new Page identity.", cause)
        }
      }

      await loadRecovery()

      let opened: SharedPage | undefined
      try {
        const response = await http.post<SharedPage>("/api/page-drafts", {
          pageId: page?.id,
          submissionId,
        }, { notifyOnError: false, signal: requests.signal })
        if (!response.ok) {
          throw response.error
        }
        opened = response.data
      } catch (cause) {
        if (!page || recovered.length === 0) {
          throw cause
        }
        fail(cause)
      }
      if (closed) {
        return
      }

      const pageId = opened?.pageId || page!.id
      const document = new Y.Doc()
      if (opened) {
        Y.applyUpdate(document, decodePageState(opened.state), remote)
        serverVector = decodePageState(opened.stateVector)
      }
      const awareness = new awarenessProtocol.Awareness(document)
      collaboration = { document, awareness, pageId }

      applyRecovery()

      retained = (opened?.legacyDrafts || []).map(saved => ({
        id: `server:${saved.id}`,
        revision: 0,
        kind: "page" as const,
        phase: "editable" as const,
        scope: `page:${pageId}`,
        updatedAt: new Date(saved.updatedAt).getTime(),
        payload: {
          title: saved.title,
          slug: saved.slug,
          content: saved.content,
          excerpt: saved.excerpt || "",
          metaDescription: saved.metaDescription || "",
          showToc: saved.showToc,
          bannerImage: saved.bannerImageBKey ? { kind: "stored" as const, bkey: saved.bannerImageBKey } : null,
        },
      }))
      setAlternatives(old => [...old, ...retained])
      if (closed) {
        awareness.destroy()
        document.destroy()
        return
      }

      const onUpdate = (_update: Uint8Array, source: unknown) => {
        if (source !== remote) {
          localVersion++
          accountDrafts.markUnsaved(receipt.id, true)
          persist()
          schedule()
        }
        if (!closed) {
          setValue(current())
        }
      }
      document.on("update", onUpdate)
      setValue(current())
      setSession(collaboration)
      if (!page) {
        window.history.replaceState(null, "", `/admin/pages/edit/${collaboration.pageId}`)
        try {
          sessionStorage.removeItem(creationKey)
        } catch (cause) {
          console.error("Could not remove the saved Page creation identity.", cause)
        }
      }

      actions.current = {
        change(change) {
          const changes = typeof change === "function" ? change(current()) : change
          const previousBanner = pendingBanner
          if (changes.bannerImage?.kind === "local" || changes.bannerImage?.kind === "missing") {
            pendingBanner = changes.bannerImage
          } else if (changes.bannerImage !== undefined) {
            pendingBanner = undefined
          }

          changePageDocument(document, changes, origin)
          if (pendingBanner !== previousBanner) {
            localVersion++
            accountDrafts.markUnsaved(receipt.id, true)
            persist()
            schedule()
          }

          setValue(current())
        },
        flush: synchronize,
        async restore(saved) {
          this.change(saved.payload)
          await synchronize()
          if (!saved.id.startsWith("server:")) {
            await accountDrafts.remove(account, saved)
          }
          setAlternatives(items => items.filter(item => item.id !== saved.id))
        },
        async retry() {
          if (recoveryReadFailed) {
            await loadRecovery()
            applyRecovery()
            setValue(current())
          }

          persist()
          await synchronize()
        },
      }

      awareness.on("update", (_changes: unknown, source: unknown) => {
        if (source !== "local") {
          return
        }

        clearTimeout(cursorTimer)
        cursorTimer = setTimeout(() => {
          const cursor = awareness.getLocalState()?.cursor
          http.post(`/api/pages/${collaboration!.pageId}/draft/cursor`, {
            clientId: document.clientID,
            field: "content",
            anchor: cursor?.anchor || null,
            head: cursor?.head || null,
          }, { notifyOnError: false, signal: requests.signal }).catch(() => {})
        }, 100)
      })

      connection = new EventSource(`/api/pages/${collaboration.pageId}/draft/events?clientId=${document.clientID}`)
      connection.onopen = () => {
        synchronize().catch(() => {})
      }
      connection.onmessage = event => {
        const message = JSON.parse(event.data)
        if (message.type === "sync") {
          synchronize().catch(() => {})
        }
        if (message.type === "update") {
          Y.applyUpdate(document, decodePageState(message.update), remote)
        }
        if (message.type === "cursor" && message.clientId !== document.clientID) {
          const color = `hsl(${(message.user.id * 137) % 360} 70% 42%)`
          const previous = awareness.meta.get(message.clientId)
          awareness.meta.set(message.clientId, { clock: (previous?.clock || 0) + 1, lastUpdated: Date.now() })
          awareness.states.set(message.clientId, {
            user: { name: message.user.name, color, colorLight: `hsl(${(message.user.id * 137) % 360} 70% 42% / 20%)` },
            cursor: message.anchor && message.head ? {
              anchor: Y.createRelativePositionFromJSON(message.anchor),
              head: Y.createRelativePositionFromJSON(message.head),
            } : null,
          })
          awareness.emit("change", [{ added: [], updated: [message.clientId], removed: [] }, remote])
        }
        if (message.type === "leave") {
          awarenessProtocol.removeAwarenessStates(awareness, [message.clientId], remote)
        }
      }
      connection.onerror = () => {
        const peers = [...awareness.getStates().keys()].filter(clientId => clientId !== document.clientID)
        awarenessProtocol.removeAwarenessStates(awareness, peers, remote)

        if (!closed) {
          setError({ errors: [{ message: "The connection was interrupted. Your edits are retained and will retry automatically." }] })
        }
      }
      if (localVersion) {
        schedule()
      }
    }

    initialize().catch(fail)
    const reconnect = () => collaboration ? synchronize().catch(() => {}) : setAttempt(++restart.current)
    window.addEventListener("online", reconnect)
    return () => {
      closed = true
      clearTimeout(timer)
      clearTimeout(cursorTimer)
      clearTimeout(retryTimer)
      window.removeEventListener("online", reconnect)
      connection?.close()
      requests.abort()
      Promise.allSettled([storing, syncing]).then(async () => {
        if (localVersion === acceptedVersion && pendingBanner === undefined) {
          await accountDrafts.remove(account, receipt).catch(cause => console.error("Could not remove acknowledged Page recovery data.", cause))
        }
        collaboration?.awareness.destroy()
        collaboration?.document.destroy()
      })
      actions.current = undefined
    }
  }, [account, page?.id, attempt])

  return {
    value, session, error, recoveryError, alternatives,
    change: (change: Partial<PageWorkingCopy> | ((current: PageWorkingCopy) => Partial<PageWorkingCopy>)) => actions.current?.change(change),
    flush: () => actions.current?.flush() || Promise.reject(new Error("The shared Page is not loaded.")),
    restore: (draft: AccountDraft<Partial<PageWorkingCopy>>) => actions.current!.restore(draft),
    retry: () => actions.current ? actions.current.retry().catch(() => {}) : setAttempt(++restart.current),
  }
}
