import { DraftImage, restoreDraftImage, SavedDraftImage } from "./draftImages"
import { readSavedDrafts } from "./importDrafts"

export type DraftKind = "post" | "comment" | "page"

export interface DraftPayload {
  attachments?: DraftImage[]
  bannerImage?: DraftImage | null
}

export interface DraftReceipt {
  id: string
  revision: number
}

export interface AccountDraft<T extends DraftPayload = DraftPayload> extends DraftReceipt {
  kind: DraftKind
  phase: "editable" | "pending"
  scope: string
  payload: T
  updatedAt: number
}

export type DraftChange<T extends DraftPayload = DraftPayload> =
  | { accepted: true; draft: AccountDraft<T> }
  | { accepted: false; draft: AccountDraft<T> | null }

export type DraftUpdate =
  | { account: string; id: string; scope?: string }
  | { account: string; expired: true }

export type DraftWrite<T extends DraftPayload> = Omit<AccountDraft<T>, "updatedAt" | "phase">

type StoredDraft = AccountDraft & { account: string }

interface DraftAccount {
  lastActiveAt?: number
  imported?: boolean
}

const draftLifetime = 3 * 24 * 60 * 60 * 1000
const activityInterval = 60 * 1000
let database: Promise<IDBDatabase> | undefined
let changes: BroadcastChannel | undefined
const initialized = new Map<string, Promise<void>>()
const unsaved = new Set<string>()

function warnBeforeUnload(event: BeforeUnloadEvent) {
  if (unsaved.size === 0) return

  event.preventDefault()
  event.returnValue = ""
}

function restoreImages(payload: { attachments?: (DraftImage | SavedDraftImage)[]; bannerImage?: DraftImage | SavedDraftImage | null }): DraftPayload {
  const restore = (image: DraftImage | SavedDraftImage) => "kind" in image ? image : restoreDraftImage(image)
  const { attachments, bannerImage, ...fields } = payload
  return {
    ...fields,
    ...(attachments && { attachments: attachments.map(restore) }),
    ...(bannerImage !== undefined && { bannerImage: bannerImage ? restore(bannerImage) : null }),
  }
}

function openDatabase(): Promise<IDBDatabase> {
  if (!database) {
    if (!changes && typeof BroadcastChannel !== "undefined") {
      changes = new BroadcastChannel("fider-drafts")
      changes.onmessage = event => window.dispatchEvent(new CustomEvent("account-drafts-changed", { detail: event.data }))
    }

    database = new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open("fider-drafts", 3)
      request.onupgradeneeded = event => {
        const accounts = event.oldVersion === 0
          ? request.result.createObjectStore("accounts")
          : request.transaction!.objectStore("accounts")
        if (event.oldVersion === 0) accounts.createIndex("lastActiveAt", "lastActiveAt")

        const drafts = event.oldVersion < 2
          ? request.result.createObjectStore("drafts", { keyPath: ["account", "id"] })
          : request.transaction!.objectStore("drafts")
        if (event.oldVersion < 2) {
          drafts.createIndex("account", "account")
          drafts.createIndex("scope", ["account", "scope"])
          const previous = accounts.openCursor()
          previous.onsuccess = () => {
            const cursor = previous.result
            if (!cursor) return

            for (const draft of cursor.value.drafts || []) {
              drafts.put({ ...draft, payload: restoreImages(draft.payload), account: cursor.primaryKey })
            }
            cursor.update({ lastActiveAt: cursor.value.lastActiveAt, imported: cursor.value.imported })
            cursor.continue()
          }
        } else {
          const previous = drafts.openCursor()
          previous.onsuccess = () => {
            const cursor = previous.result
            if (!cursor) return

            cursor.update({ ...cursor.value, payload: restoreImages(cursor.value.payload) })
            cursor.continue()
          }
        }
      }
      request.onsuccess = () => {
        const connection = request.result
        connection.onversionchange = () => {
          connection.close()
          database = undefined
        }
        resolve(connection)
      }
      request.onerror = () => reject(request.error)
    }).catch(cause => {
      database = undefined
      throw cause
    })
  }

  return database
}

function removeAccountDrafts(store: IDBObjectStore, account: IDBValidKey, done: () => void) {
  const request = store.index("account").openKeyCursor(IDBKeyRange.only(account))
  request.onsuccess = () => {
    const cursor = request.result
    if (!cursor) {
      done()
      return
    }

    store.delete(cursor.primaryKey)
    cursor.continue()
  }
}

async function access<T>(
  account: string,
  operation: (store: IDBObjectStore, finish: (result: T) => void, metadata: DraftAccount) => void
): Promise<T> {
  const connection = await openDatabase()
  return new Promise<T>((resolve, reject) => {
    const transaction = connection.transaction(["accounts", "drafts"], "readwrite")
    const accounts = transaction.objectStore("accounts")
    const drafts = transaction.objectStore("drafts")
    const read = accounts.get(account)
    const now = Date.now()
    const expiredAccounts: string[] = []
    let result: T

    const expired = accounts.index("lastActiveAt").openCursor(IDBKeyRange.upperBound(now - draftLifetime))
    expired.onsuccess = () => {
      const cursor = expired.result
      if (!cursor) return

      if (cursor.primaryKey !== account) {
        expiredAccounts.push(cursor.primaryKey as string)
        removeAccountDrafts(drafts, cursor.primaryKey, () => {})
        cursor.update({ imported: cursor.value.imported })
      }
      cursor.continue()
    }

    read.onsuccess = () => {
      const metadata: DraftAccount = read.result || {}
      const run = () => {
        try {
          operation(drafts, value => {
            result = value
            accounts.put({ ...metadata, lastActiveAt: now }, account)
          }, metadata)
        } catch (cause) {
          transaction.abort()
          reject(cause)
        }
      }

      if (metadata.lastActiveAt && metadata.lastActiveAt <= now - draftLifetime) {
        expiredAccounts.push(account)
        removeAccountDrafts(drafts, account, run)
      } else {
        run()
      }
    }

    transaction.oncomplete = () => {
      for (const account of expiredAccounts) changed({ account, expired: true })
      resolve(result)
    }
    transaction.onabort = () => reject(transaction.error)
    transaction.onerror = () => reject(transaction.error)
  })
}

function initialize(account: string): Promise<void> {
  let pending = initialized.get(account)
  if (pending) return pending

  pending = (async () => {
    if (await access(account, (_, finish, metadata) => finish(metadata.imported))) return

    const previous = await readSavedDrafts(account)
    await access(account, (store, finish, metadata) => {
      if (!metadata.imported) {
        for (const draft of previous.drafts) {
          const existing = store.get([account, draft.id])
          existing.onsuccess = () => {
            if (!existing.result) store.put({ ...draft, account })
          }
        }
        metadata.imported = true
      }
      finish(undefined)
    })
    await previous.cleanup().catch(cause => console.error("Could not remove imported drafts.", cause))
  })().catch(cause => {
    initialized.delete(account)
    throw cause
  })
  initialized.set(account, pending)
  return pending
}

function changed(detail: DraftUpdate) {
  window.dispatchEvent(new CustomEvent("account-drafts-changed", { detail }))
  changes?.postMessage(detail)
}

export const accountDrafts = {
  markUnsaved(id: string, pending: boolean) {
    if (pending) unsaved.add(id)
    else unsaved.delete(id)

    if (unsaved.size > 0) window.addEventListener("beforeunload", warnBeforeUnload)
    else window.removeEventListener("beforeunload", warnBeforeUnload)
  },

  async list(account: string, scope: string): Promise<AccountDraft[]> {
    await initialize(account)
    const saved = await access<StoredDraft[]>(account, (store, finish) => {
      const request = store.index("scope").getAll([account, scope])
      request.onsuccess = () => finish(request.result)
    })
    return saved.map(({ account: _, ...draft }) => draft).sort((left, right) => right.updatedAt - left.updatedAt)
  },

  async save<T extends DraftPayload>(account: string, draft: DraftWrite<T>, replaces?: DraftReceipt): Promise<DraftChange<T>> {
    await initialize(account)
    const result = await access<DraftChange<T>>(account, (store, finish) => {
      const request = store.get([account, draft.id])
      request.onsuccess = () => {
        const current: AccountDraft<T> | undefined = request.result
        if ((current?.revision || 0) !== draft.revision || current?.phase === "pending") {
          finish({ accepted: false, draft: current || null })
          return
        }

        const saved: AccountDraft<T> = { ...draft, phase: "editable", revision: draft.revision + 1, updatedAt: Date.now() }
        store.put({ ...saved, account })
        if (replaces && replaces.id !== draft.id) {
          const previous = store.get([account, replaces.id])
          previous.onsuccess = () => {
            if (previous.result?.revision === replaces.revision) store.delete([account, replaces.id])
          }
        }
        finish({ accepted: true, draft: saved })
      }
    })
    if (result.accepted) changed({ account, id: draft.id, scope: draft.scope })
    return result
  },

  async seal<T extends DraftPayload>(account: string, receipt: DraftReceipt): Promise<DraftChange<T>> {
    await initialize(account)
    let sealed = false
    const result = await access<DraftChange<T>>(account, (store, finish) => {
      const request = store.get([account, receipt.id])
      request.onsuccess = () => {
        if (!request.result) return finish({ accepted: false, draft: null })

        const { account: _, ...current } = request.result as StoredDraft & AccountDraft<T>
        if (current.phase === "pending" && current.revision === receipt.revision + 1) {
          finish({ accepted: true, draft: current })
          return
        }

        if (current.phase !== "editable" || current.revision !== receipt.revision) {
          finish({ accepted: false, draft: current })
          return
        }

        const draft: AccountDraft<T> = { ...current, phase: "pending", revision: current.revision + 1, updatedAt: Date.now() }
        store.put({ ...draft, account })
        sealed = true
        finish({ accepted: true, draft })
      }
    })
    if (sealed && result.accepted) changed({ account, id: result.draft.id, scope: result.draft.scope })
    return result
  },

  async resume<T extends DraftPayload>(account: string, id: string): Promise<AccountDraft<T> | null> {
    await initialize(account)
    return access(account, (store, finish) => {
      const request = store.get([account, id])
      request.onsuccess = () => {
        if (!request.result) return finish(null)

        const { account: _, ...draft } = request.result
        finish(draft)
      }
    })
  },

  async remove(account: string, receipt: DraftReceipt): Promise<boolean> {
    const removed = await access<boolean>(account, (store, finish) => {
      const request = store.get([account, receipt.id])
      request.onsuccess = () => {
        const matches = request.result?.revision === receipt.revision
        if (matches) store.delete([account, receipt.id])
        finish(matches)
      }
    })
    if (removed) changed({ account, id: receipt.id })
    return removed
  },

  async adoptAnonymous(account: string, kind: DraftKind, scope: string): Promise<void> {
    const source = `${account.split(":")[0]}:anonymous`
    await initialize(source)
    await initialize(account)
    await access(source, (_, finish) => finish(undefined))

    await access(account, (store, finish) => {
      const request = store.index("scope").getAll([source, scope])
      request.onsuccess = () => {
        for (const draft of request.result as StoredDraft[]) {
          if (draft.kind !== kind || draft.scope !== scope) continue

          const target = store.get([account, draft.id])
          target.onsuccess = () => {
            if (!target.result) store.put({ ...draft, account })
            store.delete([source, draft.id])
          }
        }
        finish(undefined)
      }
    })
  },

  trackActivity(account: string): () => void {
    let lastTouch = 0
    const touch = () => {
      const now = Date.now()
      if (now - lastTouch < activityInterval) return

      lastTouch = now
      void access(account, (_, finish) => finish(undefined)).catch(cause => {
        lastTouch = 0
        console.error("Could not retain draft activity.", cause)
      })
    }

    touch()
    window.addEventListener("pointerdown", touch)
    window.addEventListener("keydown", touch)
    return () => {
      window.removeEventListener("pointerdown", touch)
      window.removeEventListener("keydown", touch)
    }
  },
}
