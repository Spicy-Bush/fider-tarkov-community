import { test, expect, beforeEach, afterEach } from "@jest/globals"
import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { noSessionPermissions } from "@fider/services/testing/permissions"
import { PostInput } from "./PostInput"
import { actions } from "@fider/services"
import { analytics } from "@fider/services/analytics"
import { AccountDraft } from "@fider/services/browserDrafts"
import { DraftImage } from "@fider/services/draftImages"
import { RequestError } from "@fider/services/http"
import { i18n } from "@lingui/core"
import { useFider } from "@fider/hooks"
import { FiderSession } from "@fider/services/fider"

jest.mock("@lingui/react", () => ({ Trans: ({ children, message }: any) => <>{children || message}</> }))
jest.mock("@fider/hooks", () => ({ useFider: jest.fn() }))
jest.mock("@fider/contexts/UserStandingContext", () => ({ useUserStanding: () => ({ isMuted: false }) }))
jest.mock("@fider/components/common/DraftPicker", () => ({ DraftPicker: () => null }))
jest.mock("./PreviewPostModal", () => ({
  PreviewPostModal: ({ isOpen, attachments }: any) => isOpen ? <div role="dialog">Preview: {attachments.length} images</div> : null,
}))
jest.mock("@fider/services/actions/post", () => ({
  createPost: (...args: any[]) => jest.requireMock("@fider/services").actions.createPost(...args),
}))
jest.mock("@fider/services/analytics", () => ({ analytics: { event: jest.fn() } }))
jest.mock("@fider/services", () => ({ actions: { createPost: jest.fn() }, classSet: () => "" }))

const title = "A complete community suggestion"
const description = "A useful description for the community. ".repeat(6)
const mockInitial = { title, description, attachments: [] }
const mockReadUploads = jest.fn<Promise<DraftImage[]>, []>()
const mockComplete = jest.fn<Promise<boolean>, [unknown]>()
const mockReopen = jest.fn<Promise<void>, [unknown, unknown]>()
let mockNextIdentity = 0
let mockPending: AccountDraft<any>[] = []
let mockStorageError = false
let mockLoaded = true

jest.mock("@fider/hooks/useAccountDraft", () => ({
  useAccountDraft: () => {
    const React = jest.requireActual("react")
    const [value, setValue] = React.useState(mockInitial)
    const latest = React.useRef(value)
    const change = (update: any) => {
      latest.current = typeof update === "function" ? update(latest.current) : { ...latest.current, ...update }
      setValue(latest.current)
    }
    return {
      value, status: mockStorageError ? "error" : "idle", error: "Browser storage unavailable",
      loaded: mockLoaded, pending: mockPending, alternatives: [], restored: 0, change,
      flush: async () => latest.current,
      seal: async () => {
        const attachments = await mockReadUploads()
        const id = `operation-${++mockNextIdentity}`
        return { payload: { ...latest.current, attachments, submissionId: id }, draft: { id, revision: 1 } }
      },
      reopen: async (next: any, receipt: any) => {
        change(next)
        await mockReopen(next, receipt)
      },
      complete: mockComplete,
      reset: () => change(mockInitial),
    }
  },
}))

jest.mock("@fider/components", () => ({
  SignInModal: ({ isOpen }: any) => isOpen ? <div role="dialog">Sign in</div> : null,
  Button: jest.requireActual("@fider/components/common/Button").Button,
  Form: ({ children, error }: any) => (
    <div>
      {error?.errors?.map((item: any) => <p key={item.message}>{item.message}</p>)}
      {children}
    </div>
  ),
  Input: ({ field, value, onChange, disabled }: any) => (
    <input aria-label={field} value={value} disabled={disabled} onChange={event => onChange(event.target.value)} />
  ),
  TextArea: ({ field, value, onChange, disabled }: any) => (
    <textarea aria-label={field} value={value} disabled={disabled} onChange={event => onChange(event.target.value)} />
  ),
  MultiImageUploader: ({ value }: any) => <span data-testid="attachments">{value.length}</span>,
}))

const originalLocation = window.location

beforeEach(() => {
  i18n.load("en", { "home.postinput.description.placeholder": "Describe your suggestion..." })
  i18n.activate("en")
  jest.useFakeTimers()
  jest.clearAllMocks()
  sessionStorage.clear()
  mockNextIdentity = 0
  mockPending = []
  mockStorageError = false
  mockLoaded = true
  mockReadUploads.mockResolvedValue([])
  mockComplete.mockResolvedValue(true)
  mockReopen.mockResolvedValue(undefined)

  jest.mocked(useFider).mockReturnValue({
    isReadOnly: false,
    session: new FiderSession({ permissions: { ...noSessionPermissions, createPosts: true }, tenant: { id: 1 }, user: { id: 1, role: "administrator" } }),
  } as ReturnType<typeof useFider>)
  Object.defineProperty(window, "location", { configurable: true, value: { href: "http://localhost/", search: "" } })
})

afterEach(() => {
  jest.useRealTimers()
  jest.restoreAllMocks()
  Object.defineProperty(window, "location", { configurable: true, value: originalLocation })
})

async function showForm() {
  render(<PostInput placeholder="Title" onTitleChanged={() => {}} />)
  await act(async () => {})
}

async function sendNow() {
  fireEvent.click(screen.getByRole("button", { name: "Submit" }))
  fireEvent.click(screen.getByRole("button", { name: "Send Now" }))
  await act(async () => {})
}

test("signed-out visitors can draft and preview before signing in", async () => {
  jest.mocked(useFider).mockReturnValue({ session: new FiderSession({ tenant: { id: 1 } }) } as ReturnType<typeof useFider>)
  await showForm()

  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Preview" })) })
  expect(screen.getByRole("dialog")).toHaveTextContent("Preview")
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Submit" })) })
  expect(screen.getAllByRole("dialog").some(element => element.textContent === "Sign in")).toBe(true)
  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByLabelText("title")).toBeEnabled()
})

test("canceling the countdown preserves text and sends nothing", async () => {
  await showForm()
  fireEvent.click(screen.getByRole("button", { name: "Submit" }))
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
  await act(async () => { jest.advanceTimersByTime(30000) })

  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByLabelText("title")).toHaveValue(title)
})

test("manual send and countdown share one operation with the latest draft", async () => {
  let finish!: (result: any) => void
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(resolve => { finish = resolve }))
  await showForm()
  fireEvent.click(screen.getByRole("button", { name: "Submit" }))

  for (let second = 0; second < 29; second++) {
    await act(async () => { jest.advanceTimersByTime(1000) })
  }
  fireEvent.change(screen.getByLabelText("title"), { target: { value: title + " updated" } })
  fireEvent.click(screen.getByRole("button", { name: "Send Now" }))
  await act(async () => { jest.advanceTimersByTime(1000) })

  expect(actions.createPost).toHaveBeenCalledTimes(1)
  expect(actions.createPost).toHaveBeenCalledWith({ title: title + " updated", description, attachments: [], submissionId: "operation-1" }, expect.any(AbortSignal))
  expect(screen.getByLabelText("title")).toBeDisabled()
  await act(async () => finish({ ok: true, data: { number: 7, slug: "saved" } }))

  expect(window.location.href).toBe("/posts/7/saved")
  expect(mockComplete).toHaveBeenCalledWith({ id: "operation-1", revision: 1 })
  expect(analytics.event).toHaveBeenCalledTimes(1)
  expect(sessionStorage.getItem("PostInput-Submission")).toBeNull()
})

test("image preparation completes before the immutable payload is sent", async () => {
  let finishRead!: (uploads: DraftImage[]) => void
  mockReadUploads.mockImplementation(() => new Promise(resolve => { finishRead = resolve }))
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))
  await showForm()
  await sendNow()

  expect(actions.createPost).not.toHaveBeenCalled()
  const attachments = [{ remove: false, bkey: "saved.png" }]
  await act(async () => finishRead([{ kind: "stored", bkey: "saved.png" }]))
  expect(actions.createPost).toHaveBeenCalledWith({ title, description, attachments, submissionId: "operation-1" }, expect.any(AbortSignal))
})

test("unreadable images remain editable and a corrected draft can submit", async () => {
  mockReadUploads.mockResolvedValueOnce([{ kind: "missing", fileId: "lost", fileName: "lost.png" }])
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))
  await showForm()
  await sendNow()

  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByText("An attachment could not be read. Check your images and retry.")).toBeVisible()
  expect(screen.getByLabelText("title")).toBeEnabled()
  await sendNow()
  expect(actions.createPost).toHaveBeenCalledTimes(1)
})

test("reload automatically retries only the selected pending operation", async () => {
  mockPending = [{
    id: "saved-operation", revision: 3, kind: "post", phase: "pending", scope: "post:new", updatedAt: Date.now(),
    payload: { title: "Saved title", description, attachments: [] },
  }]
  sessionStorage.setItem("PostInput-Submission", "saved-operation")
  jest.mocked(actions.createPost)
    .mockRejectedValueOnce(new RequestError("POST", "/api/posts", "transport", new Error("Lost acknowledgement")))
    .mockResolvedValueOnce({ ok: true, data: { number: 7, slug: "saved" } })
  await showForm()
  await act(async () => { jest.advanceTimersByTime(500) })

  expect(actions.createPost).toHaveBeenCalledTimes(2)
  for (const [payload] of jest.mocked(actions.createPost).mock.calls) {
    expect(payload).toEqual({ submissionId: "saved-operation", title: "Saved title", description, attachments: [] })
  }
  expect(window.location.href).toBe("/posts/7/saved")
})

test("a pending operation from another tab does not replace the editable draft", async () => {
  mockPending = [{
    id: "other", revision: 1, kind: "post", phase: "pending", scope: "post:new", updatedAt: Date.now(),
    payload: { title: "Other", description: "Other", attachments: [] },
  }]
  await showForm()

  expect(screen.getByLabelText("title")).toHaveValue(title)
  expect(actions.createPost).not.toHaveBeenCalled()
})

test("slow recovery does not replace text typed since opening the editor", async () => {
  mockLoaded = false
  sessionStorage.setItem("PostInput-Submission", "selected")
  const view = render(<PostInput placeholder="Title" onTitleChanged={() => {}} />)
  fireEvent.change(screen.getByLabelText("title"), { target: { value: "New text typed during recovery" } })

  mockPending = [{
    id: "selected", revision: 1, kind: "post", phase: "pending", scope: "post:new", updatedAt: Date.now(),
    payload: { title: "Earlier submission", description, attachments: [] },
  }]
  mockLoaded = true
  view.rerender(<PostInput placeholder="Title" onTitleChanged={() => {}} />)
  await act(async () => {})

  expect(screen.getByLabelText("title")).toHaveValue("New text typed during recovery")
  expect(actions.createPost).not.toHaveBeenCalled()
})

test("a first authentication rejection preserves editable post text", async () => {
  jest.mocked(actions.createPost)
    .mockResolvedValueOnce({ ok: false, status: 401, error: { errors: [{ message: "Sign in to continue." }] } })
    .mockResolvedValueOnce({ ok: true, data: { number: 7, slug: "saved" } })
  await showForm()
  await sendNow()

  expect(screen.getByRole("dialog")).toHaveTextContent("Sign in")
  expect(screen.getByLabelText("title")).toBeEnabled()
  await sendNow()

  const calls = jest.mocked(actions.createPost).mock.calls
  expect(calls[1][0]).toMatchObject({ title, description })
  expect(calls[1][0].submissionId).not.toBe(calls[0][0].submissionId)
  expect(window.location.href).toBe("/posts/7/saved")
})

test.each([true, false, "unavailable"])("confirmed content navigates when completion result is %s", async completion => {
  mockStorageError = true
  if (completion === "unavailable") {
    mockComplete.mockRejectedValue(new Error("Storage unavailable"))
    jest.spyOn(console, "error").mockImplementation(() => {})
  } else {
    mockComplete.mockResolvedValue(completion)
  }
  jest.mocked(actions.createPost).mockResolvedValue({ ok: true, data: { number: 7, slug: "saved" } })
  await showForm()
  await sendNow()

  expect(actions.createPost).toHaveBeenCalledTimes(1)
  expect(window.location.href).toBe("/posts/7/saved")
  expect(analytics.event).toHaveBeenCalledTimes(completion === true ? 1 : 0)
})

test("validation reopens the draft even when browser storage remains unavailable", async () => {
  mockReopen.mockRejectedValue(new Error("Storage unavailable"))
  jest.spyOn(console, "error").mockImplementation(() => {})
  jest.mocked(actions.createPost).mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ message: "A clearer title is required." }] } })
  await showForm()
  await sendNow()

  expect(screen.getByLabelText("title")).toBeEnabled()
  expect(screen.getByText("A clearer title is required.")).toBeVisible()
  fireEvent.change(screen.getByLabelText("title"), { target: { value: title + " corrected" } })
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))
  await sendNow()
  expect(jest.mocked(actions.createPost).mock.calls[1][0]).toEqual(expect.objectContaining({ submissionId: "operation-2", title: title + " corrected" }))
})
