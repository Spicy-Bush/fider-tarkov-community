import React, { useEffect, useState } from "react"
import { Button, Loader, Moment } from "@fider/components"
import { Fider } from "@fider/services/fider"
import { SponsorCampaign, SponsorPlacement, SponsorReport } from "@fider/models/sponsorBooking"
import { getSponsorReport } from "@fider/services/actions/sponsorBooking"
import { RequestError } from "@fider/services/http"
import * as notify from "@fider/services/notify"

export function BookingReport({ campaign, placements }: { campaign: SponsorCampaign; placements: SponsorPlacement[] }) {
  const [report, setReport] = useState<SponsorReport>()
  const [failed, setFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    setFailed(false)
    const load = async () => {
      try {
        const result = await getSponsorReport(campaign.id)
        if (!active) return

        if (result.ok) {
          setReport(result.data)
        } else {
          setFailed(true)
          notify.error(result.error.errors?.map(item => item.message).join(" ") || "Could not load the allocation report.")
        }
      } catch (cause) {
        if (!(cause instanceof RequestError)) throw cause

        if (active) {
          setFailed(true)
          notify.error("Could not load the allocation report.")
        }
      }
    }

    void load()
    return () => { active = false }
  }, [campaign.id, attempt])

  const download = () => {
    const rows = [["Allocation day", "Placement", "Creative", "Eligible allocations", "Sponsor allocations", "Allocated share", "Expected allocations", "Clicks"]]
    for (const row of report!.allocations) {
      rows.push([
        row.day,
        row.placementId,
        String(row.creativeId),
        String(row.eligible),
        String(row.allocated),
        row.eligible ? `${(100 * row.allocated / row.eligible).toFixed(2)}%` : "",
        (row.expectedHundredths / 100).toFixed(2),
        String(row.clicks),
      ])
    }

    const csv = rows.map(row => row.map(value => `"${value.replace(/"/g, '""')}"`).join(",")).join("\r\n")
    const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }))
    const link = document.createElement("a")
    link.href = url
    link.download = `sponsorship-${campaign.id}.csv`
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <section className="rounded-panel border border-border p-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="m-0 text-lg font-semibold">Delivery and clicks</h2>
        {report && <Button onClick={download}>Download CSV</Button>}
      </div>
      {failed && <Button onClick={() => setAttempt(attempt + 1)}>Retry report</Button>}
      {!report && !failed && <Loader />}
      {report && (
        <>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="p-2">Allocation day</th>
                  <th className="p-2">Placement</th>
                  <th className="p-2">Creative</th>
                  <th className="p-2">Eligible</th>
                  <th className="p-2">Allocated</th>
                  <th className="p-2">Share</th>
                  <th className="p-2">Clicks</th>
                </tr>
              </thead>
              <tbody>
                {report.allocations.map(row => (
                  <tr key={`${row.day}:${row.placementId}:${row.creativeId}`} className="border-b border-border">
                    <td className="p-2">{row.day}</td>
                    <td className="p-2">{placements.find(item => item.id === row.placementId)?.name}</td>
                    <td className="p-2">{row.creativeId}</td>
                    <td className="p-2 tabular-nums">{row.eligible.toLocaleString()}</td>
                    <td className="p-2 tabular-nums">{row.allocated.toLocaleString()}</td>
                    <td className="p-2 tabular-nums">{row.eligible ? (row.allocated * 100 / row.eligible).toFixed(1) : "0"}%</td>
                    <td className="p-2 tabular-nums">{row.clicks.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {report.allocations.length === 0 && <p className="text-sm text-muted">No allocations yet.</p>}
          <details className="mt-4">
            <summary className="cursor-pointer text-sm font-medium">Booking history</summary>
            <ul className="mt-2 space-y-2 text-sm">
              {report.changes.map((change, index) => (
                <li key={index}>
                  <Moment locale={Fider.currentLocale} date={change.at} />: {change.state}
                </li>
              ))}
            </ul>
          </details>
        </>
      )}
    </section>
  )
}
