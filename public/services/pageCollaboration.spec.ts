jest.mock("lib0/webcrypto", () => {
  const crypto = require("node:crypto").webcrypto
  return { getRandomValues: crypto.getRandomValues.bind(crypto), subtle: crypto.subtle }
})

import * as Y from "yjs"
import { changePageDocument, decodePageState, encodePageState, readPageDocument } from "./pageCollaboration"

function copy(document: Y.Doc) {
  const result = new Y.Doc()
  Y.applyUpdate(result, Y.encodeStateAsUpdate(document))
  return result
}

test("concurrent text edits and independent selections survive duplicate and reversed delivery", () => {
  const original = new Y.Doc()
  changePageDocument(original, { title: "😀 Page", content: "First paragraph\nSecond paragraph", topics: [1] }, null)
  const first = copy(original)
  const second = copy(original)

  changePageDocument(first, { content: "First edited paragraph\nSecond paragraph", topics: [1, 2] }, "local")
  changePageDocument(second, { content: "First paragraph\nSecond revised paragraph", topics: [1, 3] }, "local")
  const firstUpdate = Y.encodeStateAsUpdate(first, Y.encodeStateVector(original))
  const secondUpdate = Y.encodeStateAsUpdate(second, Y.encodeStateVector(original))

  Y.applyUpdate(original, secondUpdate)
  Y.applyUpdate(original, firstUpdate)
  Y.applyUpdate(original, secondUpdate)
  expect(readPageDocument(original).content).toBe("First edited paragraph\nSecond revised paragraph")
  expect(readPageDocument(original).topics).toEqual([1, 2, 3])
})

test.each([
  ["😀 Page", "😃 Page"],
  ["End 😀", "End 😃"],
  ["😀", ""],
  ["", "😀"],
])("text edits preserve complete Unicode characters: %s -> %s", (before, after) => {
  const document = new Y.Doc()
  changePageDocument(document, { title: before }, null)
  changePageDocument(document, { title: after }, null)
  expect(readPageDocument(document).title).toBe(after)
})

test("binary state transport preserves a large document without argument-count overflow", () => {
  const bytes = Uint8Array.from({ length: 300_000 }, (_, index) => index % 256)
  const decoded = decodePageState(encodePageState(bytes))
  expect(decoded.length).toBe(bytes.length)
  expect(decoded.every((value, index) => value === bytes[index])).toBe(true)
})

test("text-only edits preserve author display order and restoring a draft can change it", () => {
  const document = new Y.Doc()
  changePageDocument(document, { title: "Page", authors: [9, 2, 7] }, null)

  changePageDocument(document, { content: "Only the body changed" }, "local")
  expect(readPageDocument(document).authors).toEqual([9, 2, 7])

  changePageDocument(document, { title: "Retained draft", authors: [7, 9, 2] }, "restore")
  expect(readPageDocument(document).authors).toEqual([7, 9, 2])
})

test.each([
  [[9, 2, 7, 4], [9, 2, 7, 4], [9, 2, 7, 4]],
  [[9, 2, 7, 4], [9, 2, 7, 5], [9, 2, 7, 4, 5]],
  [[9, 7], [9, 2, 7, 4], [9, 7, 4]],
  [[9, 7], [2, 7], [7]],
])("concurrent author selections preserve membership and deterministic order: %j / %j", (firstSelection, secondSelection, expected) => {
  const original = new Y.Doc()
  changePageDocument(original, { authors: [9, 2, 7] }, null)
  const first = copy(original)
  const second = copy(original)
  first.clientID = 100
  second.clientID = 200

  changePageDocument(first, { authors: firstSelection }, "local")
  changePageDocument(second, { authors: secondSelection }, "local")
  const firstUpdate = Y.encodeStateAsUpdate(first)
  const secondUpdate = Y.encodeStateAsUpdate(second)

  Y.applyUpdate(first, secondUpdate)
  Y.applyUpdate(second, firstUpdate)
  Y.applyUpdate(first, secondUpdate)
  expect(readPageDocument(first).authors).toEqual(expected)
  expect(readPageDocument(second).authors).toEqual(expected)
})
