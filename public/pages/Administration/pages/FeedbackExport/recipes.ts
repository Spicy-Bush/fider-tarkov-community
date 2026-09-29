import { FeedbackExportMode, FeedbackExportPick, FeedbackExportRecipe, FeedbackExportSection, FeedbackExportStatus, Tag } from "@fider/models"

export const PICK_LABELS: Record<FeedbackExportMode, string> = {
  top: "Top voted",
  controversial: "Controversial",
  discussed: "Most discussed",
  random: "Random",
}

export const STATUS_LABELS: Record<FeedbackExportStatus, string> = {
  open: "Open",
  planned: "Planned",
  started: "Started",
  completed: "Completed",
  declined: "Declined",
  duplicate: "Duplicate",
}

export const AGE_OPTIONS = [
  { days: 0, label: "Any time" },
  { days: 7, label: "Last 7 days" },
  { days: 30, label: "Last 30 days" },
  { days: 90, label: "Last 90 days" },
  { days: 180, label: "Last 6 months" },
  { days: 365, label: "Last year" },
]

const pick = (mode: FeedbackExportMode, count: number, minComments = 0): FeedbackExportPick => ({ mode, count, minComments })

export const newSection = (name: string, includeTags: number[] = [], excludeTags: number[] = [], picks = [pick("top", 20)]): FeedbackExportSection => ({
  name,
  includeTags,
  excludeTags,
  statuses: ["open", "planned", "started"],
  maxAgeDays: 0,
  minVotes: null,
  picks,
})

const tagsNamed = (tags: Tag[], ...names: string[]) =>
  tags.filter((tag) => names.includes(tag.name.toLowerCase()) || names.includes(tag.slug)).map((tag) => tag.id)

export interface RecipeTemplate {
  name: string
  recipe: FeedbackExportRecipe
}

export const templates = (tags: Tag[]): RecipeTemplate[] => {
  const bugs = tagsNamed(tags, "bug", "bugs")
  const qol = tagsNamed(tags, "quality of life", "qol", "quality-of-life")
  const arena = tagsNamed(tags, "arena")
  return [
    {
      name: "Weekly sheet",
      recipe: {
        sections: [
          newSection("Top bugs", bugs, [], [pick("top", 20), pick("random", 10, 10)]),
          newSection("Top quality of life", qol, [], [pick("top", 15)]),
          newSection("Most wanted", [], [...bugs, ...qol], [pick("top", 15)]),
          newSection("Arena", arena, [], [pick("top", 10)]),
        ],
      },
    },
    { name: "Top voted", recipe: { sections: [newSection("Top voted")] } },
    { name: "Controversial", recipe: { sections: [newSection("Controversial", [], [], [pick("controversial", 20)])] } },
  ]
}

export const rowCount = (recipe: FeedbackExportRecipe) =>
  recipe.sections.reduce((total, section) => total + section.picks.reduce((sum, p) => sum + p.count, 0), 0)

export const recipeSignature = (recipe: FeedbackExportRecipe) => JSON.stringify({
  sections: recipe.sections.map((section) => ({
    name: section.name,
    statuses: [...new Set(section.statuses)].sort(),
    includeTags: [...new Set(section.includeTags)].sort((a, b) => a - b),
    excludeTags: [...new Set(section.excludeTags)].sort((a, b) => a - b),
    maxAgeDays: section.maxAgeDays,
    minVotes: section.minVotes,
    picks: section.picks.map((pick) => ({ mode: pick.mode, count: pick.count, minComments: pick.minComments })),
  })),
})
