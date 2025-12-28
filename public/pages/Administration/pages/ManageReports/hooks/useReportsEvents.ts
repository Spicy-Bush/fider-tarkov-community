import { useEffect } from "react"
import { Report, ReportStatus, ReportNewEvent, ReportAssignedEvent, ReportUnassignedEvent, ReportResolvedEvent } from "@fider/models"
import { reportsEventSource } from "@fider/services"

interface UseReportsEventsConfig {
  selectedReportRef: React.MutableRefObject<Report | null>
  selectedStatusRef: React.MutableRefObject<ReportStatus | "active">
  setReports: React.Dispatch<React.SetStateAction<Report[]>>
  setSelectedReport: React.Dispatch<React.SetStateAction<Report | null>>
  setNewReportIds: React.Dispatch<React.SetStateAction<Set<number>>>
}

export const useReportsEvents = (config: UseReportsEventsConfig): void => {
  const {
    selectedReportRef,
    selectedStatusRef,
    setReports,
    setSelectedReport,
    setNewReportIds,
  } = config

  useEffect(() => {
    reportsEventSource.connect()

    const unsubNewReport = reportsEventSource.on("report.new", (_, payload) => {
      const data = payload as ReportNewEvent
      if (selectedStatusRef.current === "active" || selectedStatusRef.current === "pending") {
        setNewReportIds((prev) => new Set(prev).add(data.reportId))
      }
    })

    const unsubAssigned = reportsEventSource.on("report.assigned", (_, payload) => {
      const data = payload as ReportAssignedEvent
      const assignedUser = {
        id: data.assignedTo.userId,
        name: data.assignedTo.userName,
        avatarURL: data.assignedTo.avatarURL,
        avatarType: data.assignedTo.avatarType,
        role: data.assignedTo.role,
        status: data.assignedTo.status,
      } as Report["assignedTo"]

      setReports((prev) =>
        prev.map((r) =>
          r.id === data.reportId
            ? { ...r, status: "in_review" as ReportStatus, assignedTo: assignedUser }
            : r
        )
      )
      if (selectedReportRef.current?.id === data.reportId) {
        setSelectedReport((prev) =>
          prev ? { ...prev, status: "in_review" as ReportStatus, assignedTo: assignedUser } : prev
        )
      }
    })

    const unsubUnassigned = reportsEventSource.on("report.unassigned", (_, payload) => {
      const data = payload as ReportUnassignedEvent
      setReports((prev) =>
        prev.map((r) =>
          r.id === data.reportId
            ? { ...r, status: "pending" as ReportStatus, assignedTo: undefined }
            : r
        )
      )
      if (selectedReportRef.current?.id === data.reportId) {
        setSelectedReport((prev) =>
          prev ? { ...prev, status: "pending" as ReportStatus, assignedTo: undefined } : prev
        )
      }
    })

    const unsubResolved = reportsEventSource.on("report.resolved", (_, payload) => {
      const data = payload as ReportResolvedEvent
      if (selectedStatusRef.current === "active") {
        setReports((prev) => prev.filter((r) => r.id !== data.reportId))
        if (selectedReportRef.current?.id === data.reportId) {
          setSelectedReport(null)
        }
      } else {
        setReports((prev) =>
          prev.map((r) => (r.id === data.reportId ? { ...r, status: data.status as ReportStatus } : r))
        )
      }
    })

    return () => {
      unsubNewReport()
      unsubAssigned()
      unsubUnassigned()
      unsubResolved()
      reportsEventSource.disconnect()
    }
  }, [])
}

