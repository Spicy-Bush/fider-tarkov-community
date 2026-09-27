import React, { useCallback, useEffect, useState } from "react"
import { Button } from "@fider/components"
import { http } from "@fider/services/http"

interface Check {
  contentType: string
  contentID: number
  state: string
  lastError: string
}

interface CheckStatus {
  checks: Check[]
  enabled: boolean
  total: number
  failed: number
}

export function AutomaticChecks() {
  const [status, setStatus] = useState<CheckStatus | null>(null)
  const [error, setError] = useState("")

  const refresh = useCallback(async () => {
    try {
      const result = await http.get<CheckStatus>("/api/admin/moderation/checks")
      if (!result.ok) throw new Error()
      setStatus(result.data)
      setError("")
    } catch {
      setError("Couldn't load automatic checks. Please refresh.")
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const retry = async () => {
    try {
      const result = await http.post("/api/admin/moderation/retry", {})
      if (!result.ok) throw new Error()
      await refresh()
    } catch {
      setError("Couldn't restart failed checks. Please try again.")
    }
  }

  return (
    <details className="rounded-card border border-border p-3 text-sm shrink-0">
      <summary className="cursor-pointer">Automatic checks{status && ` · ${status.failed} need attention`}</summary>
      <div className="mt-3 space-y-2">
        {status && !status.enabled && <p>Automatic checks are disabled. Saved checks will resume when enabled.</p>}
        <p>Delayed checks retry automatically. Posts and comments stay published unless flagged.</p>
        {error && <p role="alert">{error}</p>}
        <div className="flex gap-2">
          <Button size="small" onClick={refresh}>
            Refresh
          </Button>
          <Button size="small" onClick={retry} disabled={!status?.enabled || status.failed === 0}>
            Retry failed checks
          </Button>
        </div>
        {status?.total === 0 && <p>No checks are waiting or need attention.</p>}
        {status && status.total > status.checks.length && (
          <p>
            Showing {status.checks.length} of {status.total} checks, failures first.
          </p>
        )}
        <ul className="max-h-56 overflow-auto space-y-2">
          {status?.checks.map((check) => (
            <li key={`${check.contentType}-${check.contentID}`}>
              <strong>
                {check.contentType} #{check.contentID}
              </strong>
              {" — "}
              {check.state === "failed" ? "Needs attention" : check.state === "running" ? "Checking" : "Waiting"}
              {check.lastError && <span className="block text-muted">{check.lastError}</span>}
            </li>
          ))}
        </ul>
      </div>
    </details>
  )
}