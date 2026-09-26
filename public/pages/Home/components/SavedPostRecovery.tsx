import React, { useEffect, useState } from "react"
import { Button } from "@fider/components"
import { PendingPostSubmission, postSubmissions, sendPostSubmission, SubmissionStorageError } from "@fider/services/postSubmission"
import { RequestError } from "@fider/services/http"

interface RecoveryResult {
  href?: string
  error?: string
  editable?: boolean
}

interface SavedPostRecoveryProps {
  account: string
  submissions: PendingPostSubmission[]
}

export function SavedPostRecovery({ account, submissions }: SavedPostRecoveryProps) {
  const [results, setResults] = useState<Record<string, RecoveryResult>>({})
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    const request = new AbortController()

    const recover = async () => {
      const saved = await postSubmissions.load(account).catch((cause) => {
        if (request.signal.aborted) {
          return null
        }

        if (!(cause instanceof SubmissionStorageError)) {
          throw cause
        }

        const errors = submissions.map((item) => [
          item.submissionId,
          { error: "Could not load saved submissions." },
        ])

        setResults(Object.fromEntries(errors))
        return null
      })

      if (!saved) {
        return
      }

      for (const submission of submissions) {
        if (request.signal.aborted) {
          return
        }

        const result: RecoveryResult = {}

        try {
          const stored = saved.find((item) => item.submissionId === submission.submissionId)

          if (!stored) {
            continue
          }

          if ("receipt" in stored) {
            result.href = `/posts/${stored.receipt.number}/${stored.receipt.slug}`
          } else if (stored.rejection) {
            result.error = stored.rejection.errors?.[0]?.message || "Review this submission."
            result.editable = true
          } else {
            const response = await sendPostSubmission(stored, request.signal)

            if (request.signal.aborted) {
              return
            }

            if (response.ok) {
              await postSubmissions.complete(account, submission.submissionId, response.data)
              result.href = `/posts/${response.data.number}/${response.data.slug}`
            } else if (response.status === 400) {
              const rejected = await postSubmissions.save(account, { ...stored, rejection: response.error })

              if ("receipt" in rejected) {
                result.href = `/posts/${rejected.receipt.number}/${rejected.receipt.slug}`
              } else {
                result.error = response.error.errors?.[0]?.message || "Review this submission."
                result.editable = true
              }
            } else {
              result.error = response.error.errors?.[0]?.message || "Submission not confirmed."
            }
          }
        } catch (cause) {
          if (request.signal.aborted) {
            return
          }

          if (!(cause instanceof RequestError) && !(cause instanceof SubmissionStorageError)) {
            throw cause
          }

          result.error = cause instanceof RequestError
            ? "Submission not confirmed."
            : "Could not save the submission receipt."
        }

        if (request.signal.aborted) {
          return
        }

        setResults((current) => ({ ...current, [submission.submissionId]: result }))
      }
    }

    void recover()

    const reconnect = () => setRetry((value) => value + 1)
    window.addEventListener("online", reconnect)

    return () => {
      request.abort()
      window.removeEventListener("online", reconnect)
    }
  }, [account, submissions, retry])

  if (submissions.length === 0) {
    return null
  }

  return (
    <div className="mb-3">
      {submissions.map((submission) => {
        const result = results[submission.submissionId]

        return (
          <div key={submission.submissionId}>
            {result?.href ? (
              <a href={result.href}>{submission.title}</a>
            ) : (
              <span>{submission.title}: {result?.error || "Recovering submission…"}</span>
            )}
            {result?.error && !result.editable && (
              <Button onClick={() => setRetry((value) => value + 1)}>Retry</Button>
            )}
            {result?.editable && (
              <a
                href={`/?submission=${encodeURIComponent(submission.submissionId)}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                Edit saved submission
              </a>
            )}
          </div>
        )
      })}
    </div>
  )
}
