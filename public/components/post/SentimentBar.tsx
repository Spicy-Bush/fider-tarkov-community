import React from "react"
import { Post } from "@fider/models"
import { i18n } from "@lingui/core"

interface SentimentBarProps {
  post: Post
  upvotes?: number
  downvotes?: number
}

type Sentiment = "favorite" | "wellReceived" | "controversial" | "unpopular" | "tooEarly"

const getSentiment = (up: number, down: number): Sentiment => {
  const total = up + down

  if (total < 10) return "tooEarly"

  const ratio = total > 0 ? up / total : 0.5

  if (ratio >= 0.85) return "favorite"
  if (ratio >= 0.65) return "wellReceived"
  if (ratio >= 0.4) return "controversial"
  return "unpopular"
}

const sentimentConfig: Record<Sentiment, { labelId: string; labelDefault: string; barClass: string; textClass: string }> = {
  favorite: {
    labelId: "sentiment.favorite",
    labelDefault: "Community Favorite",
    barClass: "bg-success",
    textClass: "text-success",
  },
  wellReceived: {
    labelId: "sentiment.wellreceived",
    labelDefault: "Well Received",
    barClass: "bg-primary",
    textClass: "text-primary",
  },
  controversial: {
    labelId: "sentiment.controversial",
    labelDefault: "Controversial",
    barClass: "bg-warning",
    textClass: "text-warning",
  },
  unpopular: {
    labelId: "sentiment.unpopular",
    labelDefault: "Unpopular",
    barClass: "bg-danger",
    textClass: "text-danger",
  },
  tooEarly: {
    labelId: "sentiment.tooearly",
    labelDefault: "Too Early",
    barClass: "bg-muted opacity-50",
    textClass: "text-subtle",
  },
}

export const SentimentBar: React.FC<SentimentBarProps> = ({ post, upvotes, downvotes }) => {
  const up = upvotes !== undefined ? upvotes : post.votesCount
  const down = downvotes !== undefined ? downvotes : (post.downvotes || 0)
  const total = up + down
  const ratio = total > 0 ? (up / total) * 100 : 50
  
  const sentiment = getSentiment(up, down)
  const config = sentimentConfig[sentiment]

  const label = i18n._(config.labelId, { message: config.labelDefault })
  const tooltip = i18n._("sentiment.tooltip", { message: "Post sentiment" })

  return (
    <div className="w-full cursor-help" data-tooltip={tooltip}>
      <div className="text-center mb-2">
        <span className={`text-sm font-medium transition-colors duration-300 ${config.textClass}`}>{label}</span>
      </div>
      <div className="h-1 rounded-full overflow-hidden bg-surface-alt">
        <div
          className={`h-full rounded-full transition-all duration-500 ease-out ${config.barClass}`}
          style={{ width: `${ratio}%` }}
        />
      </div>
    </div>
  )
}

