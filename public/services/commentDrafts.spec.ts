import { beforeEach, afterEach, expect, jest, test } from "@jest/globals"

const originalIndexedDB = Object.getOwnPropertyDescriptor(window, "indexedDB")

beforeEach(() => {
  jest.resetModules()
  localStorage.clear()
})

afterEach(() => {
  if (originalIndexedDB) {
    Object.defineProperty(window, "indexedDB", originalIndexedDB)
  } else {
    delete (window as any).indexedDB
  }
})

test("a synchronous open failure does not retain a rejected connection", async () => {
  const firstFailure = new DOMException("Storage temporarily unavailable", "SecurityError")
  const secondFailure = new DOMException("A fresh acquisition was attempted", "UnknownError")
  const open = jest.fn()
    .mockImplementationOnce(() => {
      throw firstFailure
    })
    .mockImplementationOnce(() => {
      throw secondFailure
    })

  Object.defineProperty(window, "indexedDB", { configurable: true, value: { open } })
  const { commentDrafts } = await import("./commentDrafts")

  await expect(commentDrafts.load("tenant:page:1:root")).rejects.toBe(firstFailure)
  await expect(commentDrafts.load("tenant:page:1:root")).rejects.toBe(secondFailure)
  expect(open).toHaveBeenCalledTimes(2)
})

test("concurrent readers share a failed opening and the next read acquires again", async () => {
  const failure = new DOMException("Storage temporarily unavailable", "UnknownError")
  let request: { error: DOMException; onerror?: () => void }
  const open = jest.fn(() => {
    request = { error: failure }
    return request
  })

  Object.defineProperty(window, "indexedDB", { configurable: true, value: { open } })
  const { commentDrafts } = await import("./commentDrafts")
  const readers = [commentDrafts.load("first"), commentDrafts.load("second")]
  const failedReaders = Promise.allSettled(readers)

  expect(open).toHaveBeenCalledTimes(1)
  request!.onerror!()
  expect(await failedReaders).toEqual([
    { status: "rejected", reason: failure },
    { status: "rejected", reason: failure },
  ])

  const next = commentDrafts.load("third")
  const nextFailure = expect(next).rejects.toBe(failure)
  expect(open).toHaveBeenCalledTimes(2)
  request!.onerror!()
  await nextFailure
})
