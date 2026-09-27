import { act, renderHook } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { i18n } from "@lingui/core"
import { actions } from "@fider/services"
import { Report } from "@fider/models"
import { useReportsState } from "./useReportsState"
import { useReportsActions } from "./useReportsActions"

jest.mock("@fider/services", () => ({
  PAGINATION: { REPORTS_LIMIT: 20 },
  Fider: { session: { user: { id: 10, name: "Reviewer" } } },
  actions: {
    getReportReasons: jest.fn(),
    listReports: jest.fn(),
    getReportDetails: jest.fn(),
    assignReport: jest.fn(),
    resolveReport: jest.fn(),
  },
}))

function pendingResponse() {
  let resolve!: (value: any) => void
  let reject!: (cause: unknown) => void
  const promise = new Promise<any>((done, fail) => {
    resolve = done
    reject = fail
  })

  return {
    promise,
    resolve: (value: any) => act(async () => {
      resolve(value)
    }),
    reject: (cause: unknown) => act(async () => {
      reject(cause)
    }),
  }
}

function report(id: number, status: Report["status"] = "pending"): Report {
  return {
    id,
    status,
    reportedType: "comment",
    reportedId: id,
    reason: "spam",
    createdAt: "2026-01-01",
    reporter: null,
  }
}

function listResult(id: number, reports: Report[] = [report(id)]) {
  return {
    ok: true,
    data: {
      reports,
      total: id,
      viewers: [{ reportId: id, viewers: [] }],
    },
  }
}

function useReportsWorkflow() {
  const state = useReportsState()
  const handlers = useReportsActions({
    ...state,
    pushState: jest.fn(),
    isNavigating: { current: false },
  })

  return { ...state, ...handlers }
}

beforeEach(() => {
  jest.resetAllMocks()
  i18n.load("en", {})
  i18n.activate("en")

  jest.mocked(actions.getReportReasons).mockResolvedValue({ ok: true, data: [] })
})

test("a superseded failure cannot end a newer load or display its error", async () => {
  const old = pendingResponse()
  const fresh = pendingResponse()
  jest.mocked(actions.listReports).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise)
  const { result } = renderHook(useReportsState)

  act(() => {
    result.current.loadReports()
  })
  act(() => {
    result.current.loadReports()
  })

  await old.reject(new Error("obsolete request failed"))
  expect(result.current.error).toBeUndefined()
  expect(result.current.isLoading).toBe(true)

  await fresh.resolve(listResult(2))
  expect(result.current.isLoading).toBe(false)
})

test("a newer refresh of the same selected report owns preview content and status", async () => {
  const old = pendingResponse()
  const fresh = pendingResponse()
  jest.mocked(actions.getReportDetails).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise)
  const { result } = renderHook(useReportsState)
  act(() => result.current.setSelectedReport(report(1)))

  act(() => {
    result.current.loadPreviewContent(1)
  })
  act(() => {
    result.current.loadPreviewContent(1)
  })

  await fresh.resolve({ ok: true, data: { report: report(1, "in_review"), comment: { id: 1, content: "new" } } })
  await old.resolve({ ok: true, data: { report: report(1), comment: { id: 1, content: "old" } } })

  expect(result.current.previewComment?.content).toBe("new")
  expect(result.current.selectedReport?.status).toBe("in_review")
})

test("changing selection and unmounting cancel pending requests", async () => {
  const old = pendingResponse()
  const listing = pendingResponse()
  jest.mocked(actions.getReportDetails).mockReturnValue(old.promise)
  jest.mocked(actions.listReports).mockReturnValue(listing.promise)
  const { result, unmount } = renderHook(useReportsState)
  act(() => result.current.setSelectedReport(report(1)))

  act(() => {
    result.current.loadPreviewContent(1)
  })
  const previewSignal = jest.mocked(actions.getReportDetails).mock.calls[0][1]!
  act(() => result.current.setSelectedReport(report(2)))
  expect(previewSignal.aborted).toBe(true)

  await old.reject(new Error("previous selection failed"))
  expect(result.current.error).toBeUndefined()

  act(() => {
    result.current.loadReports()
  })
  const listSignal = jest.mocked(actions.listReports).mock.calls[0][1]!
  unmount()
  expect(listSignal.aborted).toBe(true)
  await listing.resolve(listResult(1))
})

test.each(["pending", "resolved", "type", "reason", "page"] as const)("changing %s owns the list and starts a fresh marker baseline", async (queryPart) => {
  jest.mocked(actions.listReports)
    .mockResolvedValueOnce(listResult(1))
    .mockResolvedValueOnce(listResult(2, [report(2), report(1)]))
  const { result } = renderHook(useReportsState)

  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([])

  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([2])

  const old = pendingResponse()
  jest.mocked(actions.listReports).mockReturnValueOnce(old.promise)
  act(() => {
    result.current.loadReports({ markNew: true })
  })
  const oldSignal = jest.mocked(actions.listReports).mock.calls[2][1]!

  act(() => {
    switch (queryPart) {
      case "pending":
      case "resolved":
        result.current.setSelectedStatus(queryPart)
        break
      case "type":
        result.current.setSelectedType("comment")
        break
      case "reason":
        result.current.setSelectedReason("spam")
        break
      case "page":
        result.current.setPage(2)
        break
    }
  })

  expect(oldSignal.aborted).toBe(true)
  expect([...result.current.newReportIds]).toEqual([])
  jest.mocked(actions.listReports).mockResolvedValueOnce(listResult(3))
  await act(async () => result.current.loadReports({ markNew: true }))

  await old.resolve(listResult(2, [report(2), report(1)]))

  expect(result.current.reports.map((item) => item.id)).toEqual([3])
  expect(result.current.total).toBe(3)
  expect(result.current.viewers).toEqual([{ reportId: 3, viewers: [] }])
  expect([...result.current.newReportIds]).toEqual([])
  expect(result.current.isLoading).toBe(false)
})

test("markers follow remaining rows, while selection and assignment keep a report acknowledged", async () => {
  jest.mocked(actions.listReports)
    .mockResolvedValueOnce(listResult(1))
    .mockResolvedValueOnce(listResult(4, [report(4), report(3), report(2), report(1)]))
    .mockResolvedValueOnce(listResult(3, [report(3), report(2), report(1)]))
  jest.mocked(actions.assignReport).mockResolvedValue({ ok: true, data: {} })
  jest.mocked(actions.resolveReport).mockResolvedValue({ ok: true, data: {} })
  const { result } = renderHook(useReportsWorkflow)

  await act(async () => result.current.loadReports())
  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([4, 3, 2])

  act(() => result.current.handleSelectReport(result.current.reports[1]))
  await act(async () => result.current.handleAssign())
  act(() => result.current.handleDeselectReport())

  expect(result.current.reports.find((item) => item.id === 3)?.status).toBe("in_review")
  expect([...result.current.newReportIds]).toEqual([4, 2])

  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([2])

  act(() => result.current.removeReport(2))
  expect([...result.current.newReportIds]).toEqual([])

  act(() => result.current.handleSelectReport(result.current.reports[0]))
  await act(async () => result.current.handleResolveClick("resolved", true))

  expect(result.current.reports.map((item) => item.id)).toEqual([1])
  expect(result.current.selectedReport).toBeNull()
  expect([...result.current.newReportIds]).toEqual([])
})

test("history selection and manual refresh acknowledge rows without reviving markers", async () => {
  jest.mocked(actions.listReports)
    .mockResolvedValueOnce(listResult(1))
    .mockResolvedValueOnce(listResult(3, [report(3), report(2), report(1)]))
    .mockResolvedValueOnce(listResult(3, [report(3), report(2), report(1)]))
  const { result } = renderHook(useReportsWorkflow)

  await act(async () => result.current.loadReports())
  await act(async () => result.current.loadReports({ markNew: true }))
  act(() => result.current.setSelectedReport(result.current.reports[1]))
  act(() => result.current.setSelectedReport(null))
  expect([...result.current.newReportIds]).toEqual([3])

  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([3])

  const refresh = pendingResponse()
  jest.mocked(actions.listReports).mockReturnValueOnce(refresh.promise)
  act(() => result.current.handleRefreshNewReports())
  expect([...result.current.newReportIds]).toEqual([])

  await refresh.resolve({ ok: false, error: { errors: [{ message: "Unavailable" }] } })

  expect(result.current.reports.map((item) => item.id)).toEqual([3, 2, 1])
  expect([...result.current.newReportIds]).toEqual([])
})

test("only the latest overlapping refresh can add or remove new markers", async () => {
  jest.mocked(actions.listReports)
    .mockResolvedValueOnce(listResult(0, []))
    .mockResolvedValueOnce(listResult(1))
  const { result } = renderHook(useReportsState)

  await act(async () => result.current.loadReports())
  await act(async () => result.current.loadReports({ markNew: true }))
  expect([...result.current.newReportIds]).toEqual([1])

  const old = pendingResponse()
  const fresh = pendingResponse()
  jest.mocked(actions.listReports).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise)
  act(() => {
    result.current.loadReports({ markNew: true })
  })
  act(() => {
    result.current.loadReports({ markNew: true })
  })

  await fresh.resolve(listResult(2))
  await old.resolve(listResult(1))

  expect(result.current.reports.map((item) => item.id)).toEqual([2])
  expect([...result.current.newReportIds]).toEqual([2])
})
