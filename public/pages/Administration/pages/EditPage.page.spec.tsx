import React from "react"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { Fider, http } from "@fider/services"
import { usePageCollaboration, initialPageWorkingCopy } from "@fider/hooks/usePageCollaboration"
import EditPagePage from "./EditPage.page"

jest.mock("@fider/services/postSubmission", () => {
  let id = 0
  return { newSubmissionID: () => `publication-${++id}` }
})

jest.mock("@fider/hooks/usePageCollaboration", () => ({
  ...jest.requireActual("@fider/hooks/usePageCollaboration"),
  usePageCollaboration: jest.fn(),
}))
jest.mock("../components/page/CollaborativePageContent", () => ({
  __esModule: true,
  default: () => <textarea aria-label="Page content" value="Keep this text" readOnly />,
}))

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: { id: 1, locale: "en" } })
  const modal = document.createElement("div")
  modal.id = "root-modal"
  document.body.appendChild(modal)
})

afterEach(() => {
  document.getElementById("root-modal")?.remove()
  jest.restoreAllMocks()
})

test("failed publication keeps the accepted working copy visible and retries the server-owned draft", async () => {
  const flush = jest.fn().mockResolvedValue(undefined)
  jest.mocked(usePageCollaboration).mockReturnValue({
    value: { ...initialPageWorkingCopy(), title: "Unpublished work", content: "Keep this text" },
    session: { pageId: 14, document: {} as any, awareness: {} as any },
    error: undefined,
    recoveryError: undefined,
    alternatives: [],
    change: jest.fn(),
    flush,
    restore: jest.fn(),
    retry: jest.fn(),
  })
  const saving = jest.spyOn(http, "post")
    .mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ field: "topics", message: "Select an existing topic." }] } })
    .mockResolvedValueOnce({ ok: false, status: 503, error: { errors: [{ message: "Please retry shortly." }] } })
    .mockResolvedValueOnce({ ok: false, status: 403, error: { errors: [{ message: "Current access denied." }] } })
    .mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ message: "Validation changed." }] } })

  render(<EditPagePage topics={[]} tags={[]} roles={[]} users={[]} />)
  fireEvent.click(screen.getByRole("button", { name: "Publish" }))

  expect(await screen.findByText("Select an existing topic.")).toBeVisible()
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  expect(screen.getByPlaceholderText("Page Title")).toHaveValue("Unpublished work")
  expect(await screen.findByLabelText("Page content")).toHaveValue("Keep this text")
  expect(saving).toHaveBeenCalledWith("/api/pages/14/draft/publish", { status: "published", submissionId: expect.any(String) })

  fireEvent.click(screen.getByRole("button", { name: "Publish" }))
  await waitFor(() => expect(screen.getByText("Please retry shortly.")).toBeVisible())
  expect(flush).toHaveBeenCalledTimes(2)
  expect(saving).toHaveBeenCalledTimes(2)
  expect(saving.mock.calls[1][1].submissionId).not.toEqual(saving.mock.calls[0][1].submissionId)

  fireEvent.click(screen.getByRole("button", { name: "Retry publishing" }))
  await waitFor(() => expect(screen.getByText("Current access denied.")).toBeVisible())
  expect(saving.mock.calls[2]).toEqual(saving.mock.calls[1])

  fireEvent.click(screen.getByRole("button", { name: "Retry publishing" }))
  await screen.findByText("Validation changed.")
  expect(saving.mock.calls[3]).toEqual(saving.mock.calls[1])
  expect(screen.getByRole("button", { name: "Retry publishing" })).toBeEnabled()
})
