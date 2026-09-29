import { ImageUpload, Post } from "@fider/models"
import { createPost } from "./actions/post"
import { RetryResult, retryRequest } from "./retryRequest"

export interface PendingPostSubmission {
  submissionId: string
  title: string
  description: string
  attachments: ImageUpload[]
}

export function sendPostSubmission(
  submission: PendingPostSubmission,
  signal: AbortSignal
): Promise<RetryResult<Pick<Post, "number" | "slug">>> {
  return retryRequest(() => createPost(submission, signal), { signal })
}

export function newSubmissionID(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16))

  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")
}
