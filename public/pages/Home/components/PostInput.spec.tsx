import { test, expect, beforeEach, afterEach } from "@jest/globals"
import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { PostInput } from "./PostInput"
import { actions, cache } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { postSubmissions, SubmissionStorageError } from "@fider/services/postSubmission"
import { RequestError } from "@fider/services/http"
import { i18n } from "@lingui/core"
import { useFider } from "@fider/hooks"
import { FiderSession } from "@fider/services/fider"

jest.mock("@lingui/react", () => ({
  Trans: ({ children, message }: any) => <>{children || message}</>,
}))

jest.mock("@fider/hooks", () => ({ useFider: jest.fn() }))
jest.mock("@fider/contexts/UserStandingContext", () => ({
  useUserStanding: () => ({ isMuted: false }),
}))

jest.mock("./SavedPostRecovery", () => ({ SavedPostRecovery: () => null }))
jest.mock("./PreviewPostModal", () => ({ PreviewPostModal: () => null }))
jest.mock("@fider/services/actions/post", () => ({
  createPost: (...args: any[]) => jest.requireMock("@fider/services").actions.createPost(...args),
}))

jest.mock("@fider/services/postSubmission", () => {
  const original = jest.requireActual<typeof import("@fider/services/postSubmission")>("@fider/services/postSubmission")

  return {
    sendPostSubmission: original.sendPostSubmission,
    SubmissionStorageError: original.SubmissionStorageError,
    newSubmissionID: () => "operation-1",
    postSubmissions: {
      load: jest.fn(),
      save: jest.fn(),
      updateAttachments: jest.fn(),
      complete: jest.fn(),
    },
  }
})

jest.mock("@fider/services", () => ({
  actions: { createPost: jest.fn() },
  cache: {
    session: {
      get: jest.fn(),
      set: jest.fn(),
      remove: jest.fn(),
    },
  },
  classSet: () => "",
}))

const mockReadUploads = jest.fn<Promise<ImageUpload[] | undefined>, []>()

jest.mock("@fider/components", () => ({
  Button: jest.requireActual<typeof import("@fider/components/common/Button")>("@fider/components/common/Button").Button,
  Form: ({ children, error }: any) => (
    <div>
      {error?.errors?.map((item: any) => <p key={item.message}>{item.message}</p>)}
      {children}
    </div>
  ),
  Input: ({ field, value, onChange, disabled }: any) => (
    <input
      aria-label={field}
      value={value}
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
  TextArea: ({ field, value, onChange, disabled }: any) => (
    <textarea
      aria-label={field}
      value={value}
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
  MultiImageUploader: jest.requireActual("react").forwardRef(({ initialUploads }: any, ref: React.Ref<unknown>) => {
    const { useImperativeHandle } = jest.requireActual("react")

    useImperativeHandle(ref, () => ({ readUploads: mockReadUploads }))

    return <span data-testid="attachments">{initialUploads.length}</span>
  }),
}))

const title = "A complete community suggestion"
const description = "A useful description for the community. ".repeat(6)
const originalLocation = window.location

beforeEach(() => {
  i18n.load("en", { "home.postinput.description.placeholder": "Describe your suggestion..." })
  i18n.activate("en")
  jest.useFakeTimers()
  jest.clearAllMocks()

  jest.mocked(useFider).mockReturnValue({
    isReadOnly: false,
    session: new FiderSession({ tenant: { id: 1 }, user: { id: 1, role: "administrator" } }),
  } as ReturnType<typeof useFider>)

  Object.defineProperty(window, "location", {
    configurable: true,
    value: { href: "http://localhost/", search: "" },
  })

  jest.mocked(cache.session.get).mockImplementation((key) => {
    if (key.endsWith("Title")) {
      return title
    }

    if (key.endsWith("Description")) {
      return description
    }

    return null
  })

  mockReadUploads.mockResolvedValue([])
  jest.mocked(postSubmissions.load).mockResolvedValue([])
  jest.mocked(postSubmissions.complete).mockResolvedValue(undefined)
  jest.mocked(postSubmissions.save).mockImplementation(async (_account, submission) => submission)
})

afterEach(() => {
  jest.useRealTimers()
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

test("signed-out visitors do not access the authenticated user or saved submissions", async () => {
  jest.mocked(useFider).mockReturnValue({
    isReadOnly: false,
    session: new FiderSession({ tenant: { id: 1 } }),
  } as ReturnType<typeof useFider>)

  await showForm()

  expect(actions.createPost).not.toHaveBeenCalled()
  expect(postSubmissions.load).not.toHaveBeenCalled()
})

test("canceling countdown sends nothing and retains the draft", async () => {
  await showForm()

  fireEvent.click(screen.getByRole("button", { name: "Submit" }))
  fireEvent.click(screen.getByRole("button", { name: "Cancel" }))

  await act(async () => {
    jest.advanceTimersByTime(30000)
  })

  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByLabelText("title")).toHaveValue(title)
  expect(cache.session.remove).not.toHaveBeenCalled()
})

test("timer and manual send share one operation using the latest draft", async () => {
  let finish: (value: any) => void = () => {}

  jest.mocked(actions.createPost).mockImplementation(() => new Promise((resolve) => {
    finish = resolve
  }))

  await showForm()

  fireEvent.click(screen.getByRole("button", { name: "Submit" }))

  for (let second = 0; second < 29; second++) {
    await act(async () => {
      jest.advanceTimersByTime(1000)
    })
  }

  fireEvent.change(screen.getByLabelText("title"), { target: { value: `${title} updated` } })
  fireEvent.click(screen.getByRole("button", { name: "Send Now" }))

  await act(async () => {
    jest.advanceTimersByTime(1000)
  })

  expect(actions.createPost).toHaveBeenCalledTimes(1)
  expect(actions.createPost).toHaveBeenCalledWith({
    title: `${title} updated`,
    description,
    attachments: [],
    submissionId: "operation-1",
  }, expect.any(AbortSignal))
  expect(screen.getByLabelText("title")).toBeDisabled()

  await act(async () => {
    finish({ ok: true, data: { number: 7, slug: "saved" } })
  })

  expect(window.location.href).toBe("/posts/7/saved")
  expect(postSubmissions.complete).toHaveBeenCalledWith("1:1", "operation-1", { number: 7, slug: "saved" })
})

test("submission waits for image reads before saving or sending its immutable payload", async () => {
  let finishRead: (uploads: ImageUpload[]) => void = () => {}

  mockReadUploads.mockImplementation(() => new Promise((resolve) => {
    finishRead = resolve
  }))
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))

  await showForm()
  await sendNow()

  expect(postSubmissions.save).not.toHaveBeenCalled()
  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByLabelText("title")).toBeDisabled()

  const attachments: ImageUpload[] = [{
    remove: false,
    upload: { fileName: "draft.png", contentType: "image/png", content: "draft-image" },
  }]

  await act(async () => {
    finishRead(attachments)
  })

  const submission = { submissionId: "operation-1", title, description, attachments }

  expect(postSubmissions.save).toHaveBeenCalledWith("1:1", submission, undefined)
  expect(actions.createPost).toHaveBeenCalledWith(submission, expect.any(AbortSignal))
})

test("an unreadable image prevents sending and allows the same draft to recover", async () => {
  mockReadUploads.mockResolvedValueOnce(undefined).mockResolvedValueOnce([])
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))

  await showForm()
  await sendNow()

  expect(postSubmissions.save).not.toHaveBeenCalled()
  expect(actions.createPost).not.toHaveBeenCalled()
  expect(screen.getByText("An image could not be read. Retry or remove it before submitting.")).toBeVisible()
  expect(screen.getByLabelText("title")).toHaveValue(title)
  expect(screen.getByLabelText("title")).not.toBeDisabled()

  await sendNow()

  expect(actions.createPost).toHaveBeenCalledTimes(1)
})

test("reload resumes the saved payload and retries an uncertain response with the same identity", async () => {
  const attachments: ImageUpload[] = [{
    remove: false,
    upload: { fileName: "saved.png", contentType: "image/png", content: "saved-image" },
  }]
  const submission = { submissionId: "saved-operation", title: "Saved title", description, attachments }

  jest.mocked(postSubmissions.load).mockResolvedValue([submission])
  jest.mocked(cache.session.get).mockImplementation((key) => key.endsWith("Submission") ? "saved-operation" : null)
  jest.mocked(actions.createPost)
    .mockRejectedValueOnce(new RequestError("POST", "/api/posts", "transport", new TypeError("lost acknowledgement")))
    .mockResolvedValueOnce({ ok: true, data: { id: 7, number: 7, title: "Saved title", slug: "saved" } })

  await showForm()

  await act(async () => {
    jest.advanceTimersByTime(500)
  })

  expect(actions.createPost).toHaveBeenCalledTimes(2)

  for (const call of jest.mocked(actions.createPost).mock.calls) {
    expect(call).toEqual([submission, expect.any(AbortSignal)])
  }

  expect(window.location.href).toBe("/posts/7/saved")
})

test("failed loading cannot overwrite an unknown pending submission", async () => {
  jest.mocked(postSubmissions.load).mockRejectedValueOnce(new SubmissionStorageError(new Error("storage unavailable")))

  await showForm()

  expect(screen.getByRole("button", { name: "Submit" })).toBeDisabled()
  expect(actions.createPost).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole("button", { name: "Reload saved submission" }))
  await act(async () => {})

  expect(screen.getByRole("button", { name: "Submit" })).not.toBeDisabled()
})

test("another tab's pending submission cannot replace or clear this draft", async () => {
  jest.mocked(postSubmissions.load).mockResolvedValue([
    { submissionId: "another-tab", title: "Other draft", description: "Other content", attachments: [] },
  ])

  await showForm()

  expect(screen.getByLabelText("title")).toHaveValue(title)
  expect(screen.getByLabelText("description")).toHaveValue(description)
  expect(screen.getByRole("button", { name: "Submit" })).not.toBeDisabled()
  expect(cache.session.remove).not.toHaveBeenCalled()
  expect(actions.createPost).not.toHaveBeenCalled()
})

test("a rejected submission retains its payload for editing", async () => {
  const rejection = { errors: [{ message: "A clearer title is required." }] }

  jest.mocked(actions.createPost).mockResolvedValueOnce({ ok: false, status: 400, error: rejection })

  await showForm()
  await sendNow()

  expect(postSubmissions.save).toHaveBeenLastCalledWith("1:1", {
    submissionId: "operation-1",
    title,
    description,
    attachments: [],
    rejection,
  })
  expect(cache.session.set).toHaveBeenCalledWith("PostInput-Editing", "operation-1")
  expect(screen.getByLabelText("title")).not.toBeDisabled()
})

test("editing rejected work gets a new identity so another tab can finish its own version", async () => {
  jest.mocked(postSubmissions.load).mockResolvedValue([{
    submissionId: "rejected-operation",
    title,
    description,
    attachments: [],
    rejection: { errors: [{ message: "Correct this draft." }] },
  }])
  jest.mocked(cache.session.get).mockImplementation((key) => {
    if (key.endsWith("Submission") || key.endsWith("Editing")) {
      return "rejected-operation"
    }

    return null
  })
  jest.mocked(actions.createPost).mockImplementation(() => new Promise(() => {}))

  await showForm()

  fireEvent.change(screen.getByLabelText("title"), { target: { value: `${title} corrected` } })
  await sendNow()

  expect(postSubmissions.save).toHaveBeenCalledWith("1:1", {
    submissionId: "operation-1",
    title: `${title} corrected`,
    description,
    attachments: [],
  }, "rejected-operation")
  expect(actions.createPost).toHaveBeenCalledWith(
    expect.objectContaining({ submissionId: "operation-1", title: `${title} corrected` }),
    expect.any(AbortSignal)
  )
})

test("an edit link retains its submission identity when sibling effects update the URL", async () => {
  const attachments: ImageUpload[] = [{
    remove: false,
    upload: { fileName: "saved.png", contentType: "image/png", content: "saved-image" },
  }]

  window.location.href = "http://localhost/?submission=rejected-operation"
  window.location.search = "?submission=rejected-operation"
  jest.mocked(postSubmissions.load).mockResolvedValue([{
    submissionId: "rejected-operation",
    title: "The saved submission selected by the link",
    description: "Its saved description",
    attachments,
    rejection: { errors: [{ message: "Correct this draft." }] },
  }])

  function UpdateFilters() {
    React.useEffect(() => {
      window.location.href = "http://localhost/"
      window.location.search = ""
    }, [])

    return null
  }

  render(
    <>
      <UpdateFilters />
      <PostInput placeholder="Title" onTitleChanged={() => {}} />
    </>
  )

  await act(async () => {})

  expect(screen.getByLabelText("title")).toHaveValue("The saved submission selected by the link")
  expect(screen.getByLabelText("description")).toHaveValue("Its saved description")
  expect(screen.getByTestId("attachments")).toHaveTextContent("1")
  expect(actions.createPost).not.toHaveBeenCalled()
})

test("a malformed submission link leaves a fresh draft usable", async () => {
  window.location.search = "?submission=%"

  await showForm()

  expect(screen.getByLabelText("title")).toHaveValue(title)
  expect(screen.getByRole("button", { name: "Submit" })).not.toBeDisabled()
  expect(actions.createPost).not.toHaveBeenCalled()
})

test("a receipt saved by another tab finishes an uncertain retry without another request", async () => {
  jest.mocked(postSubmissions.save)
    .mockRejectedValueOnce(new SubmissionStorageError(new Error("storage unavailable")))
    .mockResolvedValueOnce({
      submissionId: "operation-1",
      completedAt: Date.now(),
      receipt: { number: 7, slug: "saved" },
    })

  await showForm()
  await sendNow()

  expect(actions.createPost).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole("button", { name: "Retry submission" }))
  await act(async () => {})

  expect(actions.createPost).not.toHaveBeenCalled()
  expect(window.location.href).toBe("/posts/7/saved")
})
