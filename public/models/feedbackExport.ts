export type FeedbackExportMode = "top" | "controversial" | "discussed" | "random"

export type FeedbackExportStatus = "open" | "started" | "completed" | "declined" | "planned" | "duplicate"

export interface FeedbackExportPick {
  mode: FeedbackExportMode
  count: number
  minComments: number
}

export interface FeedbackExportSection {
  name: string
  includeTags: number[]
  excludeTags: number[]
  statuses: FeedbackExportStatus[]
  maxAgeDays: number
  minVotes: number | null
  picks: FeedbackExportPick[]
}

export interface FeedbackExportRecipe {
  sections: FeedbackExportSection[]
}

export interface FeedbackExportPreset {
  id: string
  name: string
  recipe: FeedbackExportRecipe
  updatedAt: string
  updatedBy: string
}

export interface FeedbackExportLimits {
  sections: number
  picks: number
  pickSize: number
  rows: number
}

export interface FeedbackExportRow {
  number: number
  title: string
  slug: string
  votes: number
  comments: number
  tagIds: number[]
  pick: FeedbackExportMode
}
