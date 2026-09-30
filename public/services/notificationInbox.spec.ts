import { createNotificationInbox } from "./notificationInbox"
import * as actions from "./actions/notification"
import { Notification } from "@fider/models"
import { RequestError } from "./http"

jest.mock("./actions/notification", () => ({
  getUnreadCounts: jest.fn(),
  getNotifications: jest.fn(),
  markNotificationAsRead: jest.fn(),
  markAllAsRead: jest.fn(),
  purgeReadNotifications: jest.fn(),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((finish) => { resolve = finish })
  return { promise, resolve }
}

const item = (id: number, read = false) => ({ id, read, title: `Notification ${id}` } as Notification)
const page = (items: Notification[], unreadTotal: number, readTotal: number, page = 1) => ({
  ok: true as const,
  data: { notifications: items, unreadTotal, readTotal, page, perPage: 10 },
})

async function settle() {
  for (let i = 0; i < 15; i++) await Promise.resolve()
}

beforeEach(() => {
  jest.resetAllMocks()
  jest.mocked(actions.getUnreadCounts).mockResolvedValue({ ok: true, data: { total: 2, pendingReports: 3, queueCount: 4 } })
  jest.mocked(actions.getNotifications).mockImplementation(async (_page, _size, type) =>
    page(type === "unread" ? [item(1), item(2)] : [], 2, 0)
  )
})

test("an older count request cannot overwrite a refreshed tray snapshot", async () => {
  const counts = deferred<Awaited<ReturnType<typeof actions.getUnreadCounts>>>()
  jest.mocked(actions.getUnreadCounts).mockReturnValueOnce(counts.promise)
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await settle()
  const loading = inbox.refresh()

  counts.resolve({ ok: true, data: { total: 90 } })
  await loading
  expect(inbox.getSnapshot().totals).toEqual({ unread: 2, read: 0 })
  expect(inbox.getSnapshot().pages.unread.items.map((entry) => entry.id)).toEqual([1, 2])
})

test("pagination appends one ordered page and counts remain totals, not loaded lengths", async () => {
  jest.mocked(actions.getNotifications)
    .mockResolvedValueOnce(page(Array.from({ length: 10 }, (_, i) => item(i + 1)), 12, 3))
    .mockResolvedValueOnce(page([item(21, true), item(22, true), item(23, true)], 12, 3))
    .mockResolvedValueOnce(page([item(11), item(12)], 12, 3, 2))
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await inbox.refresh()
  expect(inbox.getSnapshot().totals.unread).toBe(12)
  expect(inbox.getSnapshot().pages.unread.items).toHaveLength(10)

  const first = inbox.load("unread")
  void inbox.load("unread")
  await first
  await inbox.load("unread")
  expect(inbox.getSnapshot().pages.unread.items.map((entry) => entry.id)).toEqual(Array.from({ length: 12 }, (_, i) => i + 1))
  expect(actions.getNotifications).toHaveBeenCalledTimes(3)
})

test("a lost read response refreshes both lists and keeps the actionable failure", async () => {
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await inbox.refresh()
  jest.mocked(actions.markNotificationAsRead).mockRejectedValueOnce(new RequestError("POST", "/read/1", "transport", new Error("lost reply")))
  jest.mocked(actions.getNotifications).mockImplementation(async (_page, _size, type) =>
    page(type === "unread" ? [item(2)] : [item(1, true)], 1, 1)
  )

  await inbox.markRead(1)
  await settle()
  expect(inbox.getSnapshot()).toMatchObject({ changing: false, totals: { unread: 1, read: 1 } })
  expect(inbox.getSnapshot().pages.read.items.map((entry) => entry.id)).toEqual([1])
  expect(inbox.getSnapshot().error?.cause).toBeInstanceOf(RequestError)
  await inbox.refresh()
  expect(inbox.getSnapshot().error).toBeUndefined()
})

test("read-all followed by purge uses server counts and empties the correct tab", async () => {
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await inbox.refresh()
  jest.mocked(actions.markAllAsRead).mockResolvedValueOnce({ ok: true, data: undefined })
  jest.mocked(actions.getNotifications).mockImplementation(async (_page, _size, type) =>
    page(type === "read" ? [item(1, true), item(2, true)] : [], 0, 2)
  )
  await inbox.markAllRead()
  await settle()
  expect(inbox.getSnapshot().totals).toEqual({ unread: 0, read: 2 })
  expect(inbox.getSnapshot().pages.unread.items).toEqual([])

  jest.mocked(actions.purgeReadNotifications).mockResolvedValueOnce({ ok: true, data: { purgedCount: 2 } })
  jest.mocked(actions.getNotifications).mockResolvedValue(page([], 0, 0))
  await inbox.purgeRead()
  await settle()
  expect(inbox.getSnapshot().totals).toEqual({ unread: 0, read: 0 })
  expect(inbox.getSnapshot().pages.read.items).toEqual([])
})

test("a second item clicked while the first request is pending is still marked read", async () => {
  const first = deferred<Awaited<ReturnType<typeof actions.markNotificationAsRead>>>()
  jest.mocked(actions.markNotificationAsRead).mockReturnValueOnce(first.promise).mockResolvedValueOnce({ ok: true, data: undefined })
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await inbox.refresh()

  void inbox.markRead(1)
  const second = inbox.markRead(2)
  void inbox.refresh()
  await settle()
  first.resolve({ ok: true, data: undefined })
  await second
  await settle()
  expect(actions.markNotificationAsRead.mock.calls).toEqual([[1], [2]])
  expect(inbox.getSnapshot().changing).toBe(false)
})

test("account replacement prevents old responses and queued writes from affecting the next account", async () => {
  let current = true
  const response = deferred<Awaited<ReturnType<typeof actions.getNotifications>>>()
  jest.mocked(actions.getNotifications).mockReturnValueOnce(response.promise)
  const inbox = createNotificationInbox(() => current)
  inbox.start()
  void inbox.refresh()
  await settle()
  const marking = inbox.markRead(1)
  current = false
  response.resolve(page([item(1)], 1, 0))
  await marking
  expect(actions.markNotificationAsRead).not.toHaveBeenCalled()
  expect(inbox.getSnapshot().pages.unread.items).toEqual([])
})

test("a stopped and restarted provider can load counts and recover a failed list", async () => {
  const inbox = createNotificationInbox(() => true)
  const stop = inbox.start()
  stop()
  inbox.start()
  await settle()
  expect(inbox.getSnapshot().totals.unread).toBe(2)
  jest.mocked(actions.getNotifications).mockRejectedValueOnce(new RequestError("GET", "/notifications", "transport", new Error("offline")))
  await inbox.refresh()
  expect(inbox.getSnapshot().pages.unread.loading).toBe(false)
  expect(inbox.getSnapshot().error).toBeDefined()
  await inbox.refresh()
  expect(inbox.getSnapshot().pages.unread.items).toHaveLength(2)
  expect(inbox.getSnapshot().error).toBeUndefined()
})

test("an obsolete slow badge request does not hold up opening the tray", async () => {
  const counts = deferred<Awaited<ReturnType<typeof actions.getUnreadCounts>>>()
  jest.mocked(actions.getUnreadCounts).mockReturnValueOnce(counts.promise)
  const inbox = createNotificationInbox(() => true)
  inbox.start()
  await settle()
  void inbox.refresh()
  await settle()
  expect(inbox.getSnapshot().pages.unread.items.map((entry) => entry.id)).toEqual([1, 2])
  counts.resolve({ ok: true, data: { total: 90 } })
  await settle()
  expect(inbox.getSnapshot().totals.unread).toBe(2)
})
