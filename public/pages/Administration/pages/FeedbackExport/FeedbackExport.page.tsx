import React, { useEffect, useMemo, useState } from "react"
import { Button, Dropdown, Icon, Toggle } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"
import { FeedbackExportLimits, FeedbackExportPreset, FeedbackExportRecipe, FeedbackExportRow, FeedbackExportSection, FeedbackExportStatus, Tag } from "@fider/models"
import { Fider, actions, classSet, notify, tryLocalStorageGet, tryLocalStorageSet } from "@fider/services"
import { newSubmissionID } from "@fider/services/postSubmission"
import { RequestError, Result, requestOutcome } from "@fider/services/http"
import { PresetUpdateResult } from "@fider/services/actions/feedbackExport"
import {
  heroiconsChevronDown as IconChevronDown,
  heroiconsDownload as IconDownload,
  heroiconsDuplicate as IconCopy,
  heroiconsPlus as IconPlus,
  heroiconsRefresh as IconShuffle,
} from "@fider/icons.generated"
import { SectionEditor } from "./components/SectionEditor"
import { ExportPreview } from "./components/ExportPreview"
import { PICK_LABELS, newSection, recipeSignature, rowCount, templates } from "./recipes"
import { buildSheet, toCSV, toHTML, toTSV } from "./sheet"

export const pageConfig: PageConfig = {
  title: "BSG Export",
  subtitle: "Build sheets of threads for the developers to answer",
  sidebarItem: "bsg-export",
  layoutVariant: "fullWidth",
}

interface FeedbackExportPageProps {
  tags: Tag[]
  presets: FeedbackExportPreset[]
  statuses: FeedbackExportStatus[]
  limits: FeedbackExportLimits
}

const LAST_PRESET_KEY = "fider:bsg-export:preset"

const today = () => {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`
}

const newSeed = () => Math.random().toString(36).slice(2, 10)

interface Draft {
  id: string
  name: string
  recipe: FeedbackExportRecipe
}

interface Editor {
  draft: Draft
  baseline?: Draft
}

type SaveOperation = (
  | { draft: Draft; create: true }
  | { submissionId: string; draft: Draft; create: false; baseline: Draft }
) & { uncertain?: boolean }

const fromPreset = (preset: FeedbackExportPreset): Draft => ({
  id: preset.id,
  name: preset.name,
  recipe: preset.recipe,
})

const newDraft = (name: string, recipe: FeedbackExportRecipe): Draft => ({
  id: newSubmissionID(),
  name,
  recipe,
})

const reconcilePreset = (current: Editor, operation: SaveOperation, result: PresetUpdateResult): Editor => {
  const sent = operation.draft
  if (current.draft.id !== sent.id) {
    return current
  }

  const accepted = fromPreset(result.preset)
  const original = operation.create ? sent : operation.baseline
  const rejected = result.conflicts.length > 0
  const signature = (value: Draft, field: "name" | "recipe") => field === "name" ? value.name.trim() : recipeSignature(value.recipe)
  let draft = { ...accepted }
  let baseline = { ...accepted }

  for (const field of ["name", "recipe"] as const) {
    const inputValue = signature(current.draft, field)
    const sentValue = signature(sent, field)
    const savedValue = signature(accepted, field)
    const originalValue = signature(original, field)
    const pending = inputValue !== sentValue
    const hasIntent = pending || (rejected && inputValue !== originalValue)

    if (!hasIntent) {
      continue
    }

    draft = { ...draft, [field]: current.draft[field] }
    if (inputValue === savedValue || result.conflicts.includes(field)) {
      continue
    }

    const prior = savedValue === sentValue ? accepted : rejected ? original : sent

    baseline = { ...baseline, [field]: prior[field] }
  }

  return { draft, baseline }
}

const inputClass =
  "h-10 rounded-input border border-border bg-elevated px-3 text-sm text-foreground outline-none transition-colors duration-100 focus:border-primary"

const FeedbackExportPage: React.FC<FeedbackExportPageProps> = (props) => {
  const tagsByID = useMemo(() => new Map(props.tags.map((tag) => [tag.id, tag])), [props.tags])
  const starters = useMemo(() => templates(props.tags), [props.tags])

  const [presets, setPresets] = useState(props.presets)
  const [editor, setEditor] = useState<Editor>(() => {
    const last = tryLocalStorageGet(LAST_PRESET_KEY)
    const preset = props.presets.find((item) => item.id === last) ?? props.presets[0]
    if (preset) {
      const draft = fromPreset(preset)
      return { draft, baseline: draft }
    }

    return { draft: newDraft(starters[0].name, starters[0].recipe) }
  })
  const { draft, baseline } = editor
  useEffect(() => {
    if (baseline) {
      tryLocalStorageSet(LAST_PRESET_KEY, baseline.id)
    }
  }, [baseline?.id])

  const [saving, setSaving] = useState(false)
  const [saveFailure, setSaveError] = useState<{ id: string; message: string; operation?: SaveOperation; conflict?: PresetUpdateResult }>()
  const saveError = saveFailure?.id === draft.id ? saveFailure : undefined

  const [date, setDate] = useState(today)
  const [titleRow, setTitleRow] = useState(true)
  const [seed, setSeed] = useState(newSeed)

  const selectionKey = JSON.stringify(draft.recipe.sections.map(({ name, ...section }) => section))
  const previewKey = `${selectionKey}|${seed}`
  const [preview, setPreview] = useState<{ key: string; sections: FeedbackExportRow[][] } | null>(null)
  const [previewError, setPreviewError] = useState<{ key: string; message: string }>()
  const [previewAttempt, setPreviewAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setPreviewError(undefined)
    const timer = window.setTimeout(async () => {
      try {
        const result = await actions.previewFeedbackExport(draft.recipe, seed, controller.signal)
        if (controller.signal.aborted) {
          return
        }

        if (result.ok) {
          setPreview({ key: previewKey, sections: result.data.sections })
          setPreviewError(undefined)
        } else {
          setPreviewError({ key: previewKey, message: result.error.errors?.[0]?.message || "Preview failed." })
        }
      } catch (cause) {
        if (controller.signal.aborted) {
          return
        }
        if (!(cause instanceof RequestError)) {
          throw cause
        }

        setPreviewError({ key: previewKey, message: "Preview unavailable. Check your connection and retry." })
      }
    }, 250)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [previewKey, previewAttempt])

  const current = preview?.key === previewKey
  const sheetSections = (preview?.sections ?? []).map((rows, i) => ({ name: draft.recipe.sections[i]?.name || `Section ${i + 1}`, rows }))
  const total = sheetSections.reduce((sum, section) => sum + section.rows.length, 0)
  const dirty = !baseline || draft.name.trim() !== baseline.name.trim() || recipeSignature(draft.recipe) !== recipeSignature(baseline.recipe)
  const hasRandom = draft.recipe.sections.some((section) => section.picks.some((pick) => pick.mode === "random"))

  const setRecipe = (recipe: FeedbackExportRecipe) => {
    setEditor((current) => ({ ...current, draft: { ...current.draft, recipe } }))
  }
  const setSections = (sections: FeedbackExportSection[]) => setRecipe({ sections })

  const open = (preset: FeedbackExportPreset) => {
    const next = fromPreset(preset)
    setSaveError(undefined)
    setEditor({ draft: next, baseline: next })
  }

  const save = async (operation: SaveOperation) => {
    const sent = operation.draft
    setSaving(true)
    setSaveError(undefined)

    try {
      let result: Result<PresetUpdateResult> & { unconfirmed?: boolean }
      if (operation.create) {
        const created = await actions.createFeedbackExportPreset(sent)
        result = created.ok ? { ok: true, data: { preset: created.data, conflicts: [] } } : created
      } else {
        result = await actions.updateFeedbackExportPreset(operation.submissionId, sent, operation.baseline)
      }

      if (!result.ok) {
        const uncertain = operation.uncertain || result.unconfirmed || requestOutcome(result) === "uncertain"
        setSaveError({
          id: sent.id,
          message: result.error.errors?.[0]?.message || "Preset could not be saved.",
          operation: uncertain ? { ...operation, uncertain: true } : undefined,
        })
        return
      }

      const { preset, conflicts } = result.data
      setPresets((list) => [...list.filter((item) => item.id !== preset.id), preset].sort((a, b) => a.name.localeCompare(b.name)))

      setEditor((current) => reconcilePreset(current, operation, result.data))
      if (conflicts.length > 0) {
        setSaveError({ id: sent.id, message: "This preset was changed by someone else. Choose which changes to keep.", conflict: result.data })
        return
      }

      notify.success(`Saved "${preset.name}"`)
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        throw cause
      }

      setSaveError({
        id: sent.id,
        message: `Saving "${sent.name}" could not be confirmed. Check your connection and retry.`,
        operation: { ...operation, uncertain: true },
      })
    } finally {
      setSaving(false)
    }
  }

  const saveCurrent = () => {
    const pending = saveError?.operation
    if (pending) {
      return save(pending)
    }
    if (baseline) {
      return save({ submissionId: newSubmissionID(), draft, baseline, create: false })
    }

    return save({ draft, create: true })
  }

  const saveAsNew = () => {
    const next = newDraft(draft.name, draft.recipe)
    setEditor({ draft: next })
    return save({ draft: next, create: true })
  }

  const remove = async () => {
    if (!baseline || !window.confirm(`Delete the preset "${draft.name}" for everyone?`)) {
      return
    }

    setSaving(true)
    setSaveError(undefined)
    try {
      const result = await actions.deleteFeedbackExportPreset(draft.id)
      if (!result.ok) {
        setSaveError({ id: draft.id, message: result.error.errors?.[0]?.message || "Preset could not be deleted. Retry Delete." })
        return
      }

      const rest = presets.filter((preset) => preset.id !== draft.id)
      setPresets(rest)
      setEditor((current) => {
        if (current.draft.id !== draft.id) {
          return current
        }

        if (rest[0]) {
          const next = fromPreset(rest[0])
          return { draft: next, baseline: next }
        }
        return { draft: newDraft(starters[0].name, starters[0].recipe) }
      })
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        throw cause
      }
      setSaveError({ id: draft.id, message: "Deletion could not be confirmed. Check your connection and retry Delete." })
    } finally {
      setSaving(false)
    }
  }

  const sheet = () => buildSheet(sheetSections, { baseURL: Fider.settings.baseURL, date, title: draft.name, titleRow })

  const copy = async () => {
    const rows = sheet()
    try {
      if (typeof ClipboardItem !== "undefined" && navigator.clipboard?.write) {
        await navigator.clipboard.write([
          new ClipboardItem({
            "text/html": new Blob([toHTML(rows)], { type: "text/html" }),
            "text/plain": new Blob([toTSV(rows)], { type: "text/plain" }),
          }),
        ])
      } else {
        await navigator.clipboard.writeText(toTSV(rows))
      }
      notify.success(`Copied ${total} posts. Paste into Google Sheets.`)
    } catch {
      notify.error("Copying was blocked. Use Download CSV instead.")
    }
  }

  const download = () => {
    const slug = draft.name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/(^-|-$)/g, "")
    const link = document.createElement("a")
    link.href = URL.createObjectURL(new Blob(["﻿" + toCSV(sheet())], { type: "text/csv;charset=utf-8" }))
    link.download = `bsg-export-${slug ? slug + "-" : ""}${date}.csv`
    link.click()
    URL.revokeObjectURL(link.href)
  }

  const overLimit = rowCount(draft.recipe) > props.limits.rows
  const conflict = saveError?.conflict

  return (
    <div className="flex flex-col gap-6 pb-12">
      <div className="flex flex-wrap items-center gap-2">
        <select
          aria-label="Preset"
          value={baseline ? draft.id : ""}
          onChange={(event) => {
            const preset = presets.find((p) => p.id === event.currentTarget.value)
            if (preset && (!dirty || window.confirm("Discard unsaved changes to this preset?"))) {
              open(preset)
            }
          }}
          className={`${inputClass} w-64 max-w-full min-w-0 appearance-none bg-no-repeat pr-9`}
          style={{
            backgroundImage:
              "url(\"data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e\")",
            backgroundPosition: "right 0.5rem center",
            backgroundSize: "1.25em 1.25em",
          }}
        >
          {!baseline && <option value="">Unsaved preset</option>}
          {presets.map((preset) => (
            <option key={preset.id} value={preset.id}>
              {preset.name}
            </option>
          ))}
        </select>
        <input
          aria-label="Preset name"
          value={draft.name}
          onChange={(event) => {
            const name = event.currentTarget.value
            setEditor((current) => ({ ...current, draft: { ...current.draft, name } }))
          }}
          className={`${inputClass} min-w-40 flex-1 max-w-72`}
        />
        <Button variant="primary" disabled={!dirty || !draft.name.trim() || overLimit || !!saveError?.conflict} loading={saving} onClick={saveCurrent}>
          Save
        </Button>
        {(baseline || saveError) && (
          <Button variant="secondary" disabled={saving || overLimit} onClick={saveAsNew}>
            Save as new
          </Button>
        )}
        <Dropdown
          position="right"
          renderHandle={
            <span className="flex h-10 items-center gap-1 rounded-button border border-border px-3 text-sm font-medium text-foreground transition-colors hover:border-border-strong hover:bg-tertiary">
              New
              <Icon sprite={IconChevronDown} className="h-4 w-4 text-subtle" />
            </span>
          }
        >
          {starters.map((starter) => (
            <Dropdown.ListItem
              key={starter.name}
              onClick={() => {
                if (!dirty || window.confirm("Discard unsaved changes to this preset?")) {
                  setSaveError(undefined)
                  setEditor({ draft: newDraft(starter.name, starter.recipe) })
                }
              }}
            >
              {starter.name}
            </Dropdown.ListItem>
          ))}
        </Dropdown>
        {baseline && (
          <Button variant="danger" className="ml-auto" disabled={saving} onClick={remove}>
            Delete
          </Button>
        )}
      </div>

      {saveError && (
        <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-danger">
          <span>{saveError.message}</span>
          {saveError.operation && (
            <Button size="small" disabled={saving} onClick={() => save(saveError.operation!)}>
              Retry save
            </Button>
          )}
        </div>
      )}

      {conflict && (
        <div className="rounded-card border border-warning-medium bg-elevated p-4">
          <div className="grid gap-4 sm:grid-cols-2">
            {[{ label: "Saved version", value: conflict.preset }, { label: "Your changes", value: draft }].map(({ label, value }) => (
              <div key={label} className="min-w-0">
                <h3 className="mb-2 font-semibold">{label}</h3>
                {conflict.conflicts.includes("name") && <p className="break-words">{value.name}</p>}
                {conflict.conflicts.includes("recipe") && value.recipe.sections.map((section, index) => (
                  <div key={index} className="mb-3 text-sm">
                    <p className="break-words font-medium">{section.name}</p>
                    <p className="text-muted">
                      {section.statuses.join(", ")}
                      {section.maxAgeDays > 0 && `; last ${section.maxAgeDays} days`}
                      {section.minVotes !== null && `; at least ${section.minVotes} votes`}
                      {section.includeTags.length > 0 && `; includes: ${section.includeTags.map((id) => tagsByID.get(id)?.name || id).join(", ")}`}
                      {section.excludeTags.length > 0 && `; excludes: ${section.excludeTags.map((id) => tagsByID.get(id)?.name || id).join(", ")}`}
                    </p>
                    {section.picks.map((pick, pickIndex) => (
                      <p key={pickIndex} className="text-muted">
                        {pick.count} posts ({PICK_LABELS[pick.mode]})
                        {pick.minComments > 0 && `; at least ${pick.minComments} comments`}
                      </p>
                    ))}
                  </div>
                ))}
              </div>
            ))}
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button variant="primary" disabled={saving || overLimit || !draft.name.trim()} onClick={saveCurrent}>
              Save my changes
            </Button>
            <Button disabled={saving} onClick={() => {
              setEditor((current) => ({
                ...current,
                draft: {
                  ...current.draft,
                  name: conflict.conflicts.includes("name") ? conflict.preset.name : current.draft.name,
                  recipe: conflict.conflicts.includes("recipe") ? conflict.preset.recipe : current.draft.recipe,
                },
              }))
              setSaveError(undefined)
            }}>
              Use saved changes
            </Button>
          </div>
        </div>
      )}

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
        <div className="flex flex-col gap-4">
          {draft.recipe.sections.map((section, i) => (
            <SectionEditor
              key={i}
              section={section}
              index={i}
              count={draft.recipe.sections.length}
              tags={props.tags}
              statuses={props.statuses}
              limits={props.limits}
              onChange={(next) => setSections(draft.recipe.sections.map((s, j) => (i === j ? next : s)))}
              onMove={(offset) => {
                const sections = [...draft.recipe.sections]
                ;[sections[i], sections[i + offset]] = [sections[i + offset], sections[i]]
                setSections(sections)
              }}
              onRemove={() => setSections(draft.recipe.sections.filter((_, j) => j !== i))}
            />
          ))}
          {draft.recipe.sections.length < props.limits.sections && (
            <button
              type="button"
              onClick={() => setSections([...draft.recipe.sections, newSection(`Section ${draft.recipe.sections.length + 1}`)])}
              className="flex items-center justify-center gap-1.5 rounded-card border border-dashed border-border py-3 text-sm font-medium text-muted transition-colors hover:border-border-strong hover:bg-tertiary hover:text-foreground"
            >
              <Icon sprite={IconPlus} className="h-4 w-4" />
              Add section
            </button>
          )}
        </div>

        <div className="flex flex-col gap-4 xl:sticky xl:top-4">
          <div className="flex flex-wrap items-center gap-2 rounded-card border border-border bg-elevated p-3">
            <input
              type="date"
              aria-label="Export date"
              value={date}
              onChange={(event) => setDate(event.currentTarget.value || today())}
              className={`${inputClass} h-9`}
            />
            <Toggle label="Title row" active={titleRow} onToggle={setTitleRow} />
            <div className="ml-auto flex items-center gap-2">
              {hasRandom && (
                <Button size="small" variant="tertiary" onClick={() => setSeed(newSeed())}>
                  <Icon sprite={IconShuffle} />
                  <span>Reshuffle</span>
                </Button>
              )}
              <Button size="small" variant="secondary" disabled={!current || total === 0} onClick={download}>
                <Icon sprite={IconDownload} />
                <span>CSV</span>
              </Button>
              <Button size="small" variant="primary" disabled={!current || total === 0} onClick={copy}>
                <Icon sprite={IconCopy} />
                <span>Copy for Sheets</span>
              </Button>
            </div>
          </div>

          <div className="flex items-baseline justify-between px-1 text-sm">
            <span className="text-foreground">
              <span className="tabular-nums font-semibold">{total}</span> posts in {sheetSections.length} {sheetSections.length === 1 ? "section" : "sections"}
            </span>
            {overLimit && <span className="text-danger">An export can have at most {props.limits.rows} posts.</span>}
            {!overLimit && previewError?.key === previewKey && (
              <div role="alert" className="flex items-center gap-2 text-danger">
                <span>{previewError.message}</span>
                <Button size="small" onClick={() => setPreviewAttempt((attempt) => attempt + 1)}>
                  Retry preview
                </Button>
              </div>
            )}
          </div>

          {preview ? (
            <ExportPreview sections={sheetSections} tags={tagsByID} baseURL={Fider.settings.baseURL} stale={!current} />
          ) : (
            <div className={classSet({ "flex flex-col gap-2": true })}>
              {[0, 1, 2, 3, 4].map((i) => (
                <div key={i} className="skeleton h-12" />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default FeedbackExportPage
