import { Notification } from "@fider/models"
import * as actions from "./actions/notification"
import { Failure, RequestError, Result } from "./http"

export type NotificationTab = "unread" | "read"

interface NotificationPage {
  items: Notification[]
  page: number
  loading: boolean
}

export interface NotificationInboxSnapshot {
  totals: { unread: number; read: number }
  pendingReports: number
  queueCount: number
  pages: Record<NotificationTab, NotificationPage>
  changing: boolean
  error?: Failure
}

export function createNotificationInbox(currentAccount: () => boolean) {
  let snapshot: NotificationInboxSnapshot = {
    totals: { unread: 0, read: 0 },
    pendingReports: 0,
    queueCount: 0,
    pages: {
      unread: { items: [], page: 0, loading: false },
      read: { items: [], page: 0, loading: false },
    },
    changing: false,
  }
  const listeners = new Set<() => void>()
  let active = false
  let generation = 0
  let queued = Promise.resolve()
  let requestNumber = 0
  let totalsRequest = 0
  const reads: Partial<Record<NotificationTab | "counts", AbortController>> = {}

  function publish(next: NotificationInboxSnapshot) {
    snapshot = next
    for (const listener of listeners) {
      listener()
    }
  }

  function enqueue(operation: () => Promise<void>) {
    const result = queued.then(async () => {
      if (active && currentAccount()) {
        await operation()
      }
    })
    queued = result.catch(() => undefined)
    return result
  }

  async function request<T>(send: () => Promise<Result<T>>): Promise<Result<T>> {
    try {
      return await send()
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        throw cause
      }
      return { ok: false, error: { errors: [{ message: "Notifications could not be refreshed. Please retry." }], cause } }
    }
  }

  function current(expected: number) {
    return active && currentAccount() && generation === expected
  }

  async function load(tab: NotificationTab, reset = false) {
    const previous = snapshot.pages[tab]
    if (!active || !currentAccount() || snapshot.changing || previous.loading) {
      return
    }
    if (!reset && previous.page > 0 && previous.page * 10 >= snapshot.totals[tab]) {
      return
    }
    const expected = generation
    const number = ++requestNumber
    const controller = new AbortController()
    reads[tab]?.abort()
    reads[tab] = controller
    const page = reset ? 1 : previous.page + 1
    publish({ ...snapshot, pages: { ...snapshot.pages, [tab]: { ...previous, loading: true } } })

    const result = await request(() => actions.getNotifications(page, 10, tab, { signal: controller.signal }))
    if (!current(expected)) {
      return
    }

    const before = snapshot.pages[tab]
    if (!result.ok) {
      publish({ ...snapshot, error: result.error, pages: { ...snapshot.pages, [tab]: { ...before, loading: false } } })
      return
    }

    const items = page === 1 ? [] : before.items
    const known = new Set(items.map((item) => item.id))
    publish({
      ...snapshot,
      totals: number >= totalsRequest ? { unread: result.data.unreadTotal, read: result.data.readTotal } : snapshot.totals,
      pages: {
        ...snapshot.pages,
        [tab]: { items: [...items, ...result.data.notifications.filter((item) => !known.has(item.id))], page, loading: false },
      },
    })
    totalsRequest = Math.max(totalsRequest, number)
  }

  async function refreshCounts() {
    const expected = generation
    const number = ++requestNumber
    const controller = new AbortController()
    reads.counts?.abort()
    reads.counts = controller
    if (!active || !currentAccount()) {
      return
    }

    const result = await request(() => actions.getUnreadCounts({ signal: controller.signal }))
    if (!active || !currentAccount() || reads.counts !== controller) {
      return
    }
    if (!result.ok) {
      if (current(expected)) {
        publish({ ...snapshot, error: result.error })
      }
      return
    }
    const acceptTotal = current(expected) && number >= totalsRequest
    publish({
      ...snapshot,
      totals: acceptTotal ? { ...snapshot.totals, unread: result.data.total } : snapshot.totals,
      pendingReports: result.data.pendingReports ?? 0,
      queueCount: result.data.queueCount ?? 0,
    })
    if (acceptTotal) {
      totalsRequest = number
    }
  }

  function refresh() {
    if (snapshot.changing) {
      return queued
    }
    generation++
    publish({
      ...snapshot,
      error: undefined,
      pages: {
        unread: { ...snapshot.pages.unread, loading: false },
        read: { ...snapshot.pages.read, loading: false },
      },
    })
    return Promise.all([load("unread", true), load("read", true)]).then(() => undefined)
  }

  function change(send: () => Promise<Result<unknown>>) {
    generation++
    const expected = generation
    publish({ ...snapshot, changing: true, error: undefined })

    return enqueue(async () => {
      const result = await request(send)
      if (!current(expected)) {
        return
      }

      publish({ ...snapshot, changing: false, error: result.ok ? undefined : result.error })
      // A lost response can still have changed either list.
      const failure = snapshot.error
      const refreshed = refresh()
      const refreshedGeneration = generation
      await refreshed
      if (failure && current(refreshedGeneration)) {
        publish({ ...snapshot, error: failure })
      }
    })
  }

  return {
    getSnapshot: () => snapshot,
    subscribe: (listener: () => void) => {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },
    start: () => {
      active = true
      void refreshCounts()
      return () => {
        active = false
        generation++
        for (const controller of Object.values(reads)) {
          controller.abort()
        }
      }
    },
    refresh,
    refreshCounts,
    load,
    markRead: (id: number) => change(() => actions.markNotificationAsRead(id)),
    markAllRead: () => change(actions.markAllAsRead),
    purgeRead: () => change(actions.purgeReadNotifications),
  }
}
