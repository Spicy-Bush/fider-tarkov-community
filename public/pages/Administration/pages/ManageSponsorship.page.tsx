import React, { useEffect, useMemo, useState } from "react"
import { Button, Form, Hint, Icon, Input, TextArea, Toggle, Select, SelectOption } from "@fider/components"
import { heroiconsInformationCircle as IconInformationCircle } from "@fider/icons.generated"
import { VStack, HStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"
import {
  AdPlacement,
  CampaignAssignment,
  CreativeVersion,
  PublicAd,
  SponsorshipCampaign,
  SponsorshipPackage,
  SPONSORSHIP_SLOT_SPECS,
  deriveCampaignStatus,
  CampaignDerivedStatus,
} from "@fider/models"
import { actions, Failure, notify } from "@fider/services"
import { AdSlot, FeedNativeAd } from "@fider/components/sponsorship"
import {
  browserTimeZoneLabel,
  datetimeLocalToUtcIso,
  utcToDatetimeLocalValue,
} from "@fider/components/sponsorship/datetime"

export const pageConfig: PageConfig = {
  title: "Sponsorship",
  subtitle: "Packages and house ad campaigns (no public prices)",
  sidebarItem: "sponsorship",
}

interface ManageSponsorshipPageProps {
  campaigns: SponsorshipCampaign[]
  packages: SponsorshipPackage[]
  slots: string[]
  placements?: AdPlacement[]
}

type Tab = "campaigns" | "packages" | "placements"

const emptyPkg = () => ({
  slug: "",
  name: "",
  description: "",
  slots: "feed_native",
  durationDays: 30,
  sort: 0,
})

const emptyCamp = () => {
  const start = new Date()
  const end = new Date(Date.now() + 30 * 86400000)
  return {
    name: "",
    advertiser: "",
    startAtLocal: utcToDatetimeLocalValue(start.toISOString()),
    endAtLocal: utcToDatetimeLocalValue(end.toISOString()),
    weight: 100,
    locale: "all",
    enabled: true,
    packageId: undefined as number | undefined,
    configVersion: 1,
  }
}


function FieldTip({ text }: { text: string }) {
  return (
    <span className="sponsorship-field-tip ml-1" data-tooltip={text}>
      <Icon width="15" height="15" className="block shrink-0" sprite={IconInformationCircle} />
    </span>
  )
}

/** Server is the only OCC source. Missing/invalid token is a hard fail (no || 1, no local +1). */
function requireConfigVersion(v: unknown): number {
  if (typeof v !== "number" || !Number.isInteger(v) || v <= 0) {
    throw new Error("missing configVersion from server")
  }
  return v
}

function isCreateGraphResult(data: unknown): data is {
  campaign: SponsorshipCampaign
  versions: CreativeVersion[]
  assignments: CampaignAssignment[]
  configVersion: number
} {
  return !!data && typeof data === "object" && "campaign" in (data as object) && "configVersion" in (data as object)
}

function statusBadge(status: CampaignDerivedStatus): { label: string; className: string } {
  switch (status) {
    case "active":
      return { label: "active", className: "bg-green-100 text-green-800" }
    case "scheduled":
      return { label: "scheduled", className: "bg-blue-100 text-blue-800" }
    case "ended":
      return { label: "ended", className: "bg-gray-100 text-gray-700" }
    default:
      return { label: "disabled", className: "bg-amber-100 text-amber-800" }
  }
}

const tzLabel = browserTimeZoneLabel()

const ManageSponsorshipPage: React.FC<ManageSponsorshipPageProps> = (props) => {
  const placementCatalog = props.placements?.length
    ? props.placements
    : (props.slots || []).map((id) => ({
        id,
        name: SPONSORSHIP_SLOT_SPECS[id]?.label || id,
        description: "",
        kind: "frame",
        sort: 0,
        enabled: true,
      }))
  const [tab, setTab] = useState<Tab>("campaigns")
  const [packages, setPackages] = useState<SponsorshipPackage[]>(props.packages || [])
  const [placements, setPlacements] = useState<AdPlacement[]>(placementCatalog)
  const [campaigns, setCampaigns] = useState<SponsorshipCampaign[]>(props.campaigns || [])
  const slots = placements.filter((p) => p.enabled).map((p) => p.id)

  const [pkgForm, setPkgForm] = useState(emptyPkg())
  const [editingPkgId, setEditingPkgId] = useState<number | null>(null)
  const [campForm, setCampForm] = useState(emptyCamp())
  const [editingCampId, setEditingCampId] = useState<number | null>(null)
  const [error, setError] = useState<Failure | undefined>()
  const [busy, setBusy] = useState(false)

  const [versions, setVersions] = useState<CreativeVersion[]>([])
  const [assignments, setAssignments] = useState<CampaignAssignment[]>([])
  const [versionForm, setVersionForm] = useState({ imageUrl: "", html: "", clickUrl: "https://" })
  const [assignPlacementId, setAssignPlacementId] = useState("")
  const [assignVersionId, setAssignVersionId] = useState<number | "">("")
  const [assignmentsDirty, setAssignmentsDirty] = useState(false)

  const localeOptions: SelectOption[] = [
    { value: "all", label: "all" },
    { value: "en", label: "en" },
    { value: "ru", label: "ru" },
  ]

  const loadGraph = async (campaignId: number, opts?: { forceAssignments?: boolean }) => {
    const [vRes, aRes] = await Promise.all([
      actions.listCreativeVersions(campaignId),
      actions.listCampaignAssignments(campaignId),
    ])
    setVersions(vRes.ok && vRes.data ? vRes.data : [])
    // Dirty guard: staged assignment edits must not be wiped by incidental reloads
    // (e.g. after version create). forceAssignments after Save / Edit / Conflict reload.
    if (opts?.forceAssignments || !assignmentsDirty) {
      setAssignments(aRes.ok && aRes.data ? aRes.data : [])
      if (opts?.forceAssignments) {
        setAssignmentsDirty(false)
      }
    }
  }

  useEffect(() => {
    if (editingCampId) {
      setAssignmentsDirty(false)
      setAssignVersionId("")
      void loadGraph(editingCampId, { forceAssignments: true })
    } else {
      setVersions([])
      setAssignments([])
      setAssignmentsDirty(false)
      setAssignVersionId(0)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingCampId])

  const previewAd: PublicAd | null = useMemo(() => {
    const advertiser = (campForm.advertiser || "").trim()
    if (!advertiser) return null
    const imageUrl = (versionForm.imageUrl || "").trim()
    const html = (versionForm.html || "").trim()
    const clickUrl = (versionForm.clickUrl || "").trim()
    if (!imageUrl && !html) return null
    return {
      campaignId: editingCampId || 0,
      advertiser,
      placementId: assignPlacementId || slots[0] || "sidebar_top",
      creativeVersionId: 0,
      imageUrl,
      html,
      clickPath: clickUrl || "#",
    }
  }, [campForm.advertiser, versionForm, editingCampId, assignPlacementId, slots])

  const savePackage = async () => {
    setBusy(true)
    setError(undefined)
    const body = { ...pkgForm, durationDays: Number(pkgForm.durationDays), sort: Number(pkgForm.sort) }
    const result = editingPkgId
      ? await actions.updateSponsorshipPackage(editingPkgId, body)
      : await actions.createSponsorshipPackage(body)
    setBusy(false)
    if (result.ok) {
      if (editingPkgId) {
        setPackages((prev) => prev.map((p) => (p.id === editingPkgId ? result.data : p)))
      } else {
        setPackages((prev) => [...prev, result.data])
      }
      setPkgForm(emptyPkg())
      setEditingPkgId(null)
      notify.success("Package saved")
    } else {
      setError(result.error)
    }
  }

  const removePackage = async (id: number) => {
    if (!confirm("Delete this package?")) return
    const result = await actions.deleteSponsorshipPackage(id)
    if (result.ok) {
      setPackages((prev) => prev.filter((p) => p.id !== id))
      notify.success("Package deleted")
    }
  }

  const reloadCampaigns = async () => {
    const result = await actions.listSponsorshipCampaigns()
    if (result.ok && result.data) {
      setCampaigns(result.data)
      if (editingCampId) {
        const fresh = result.data.find((c) => c.id === editingCampId)
        if (fresh) {
          try {
            const cfg = requireConfigVersion(fresh.configVersion)
            setCampForm((prev) => ({ ...prev, configVersion: cfg }))
          } catch {
            notify.error("Server response missing configVersion - reload the page")
          }
        }
      }
    }
  }

  const saveCampaign = async () => {
    setBusy(true)
    setError(undefined)
    let startAt: string
    let endAt: string
    try {
      startAt = datetimeLocalToUtcIso(campForm.startAtLocal)
      endAt = datetimeLocalToUtcIso(campForm.endAtLocal)
    } catch {
      setBusy(false)
      setError({ errors: [{ message: "Invalid start/end datetime" }] })
      return
    }
    if (editingCampId) {
      let configVersion: number
      try {
        configVersion = requireConfigVersion(campForm.configVersion)
      } catch {
        setBusy(false)
        notify.error("Missing configVersion - reload the campaign before saving")
        return
      }
      // Assignment changes ONLY via graph PUT (never slim PUT).
      const result = await actions.saveSponsorshipCampaignGraph(editingCampId, {
        name: campForm.name,
        advertiser: campForm.advertiser,
        startAt,
        endAt,
        weight: Number(campForm.weight),
        locale: campForm.locale || "all",
        enabled: campForm.enabled,
        packageId: campForm.packageId || undefined,
        configVersion,
        assignments: assignments.map((a) => ({
          placementId: a.placementId,
          creativeVersionId: a.creativeVersionId,
        })),
      })
      setBusy(false)
      if (result.ok) {
        const camp = result.data.campaign
        try {
          const cfg = requireConfigVersion(camp.configVersion)
          setCampaigns((prev) => prev.map((c) => (c.id === editingCampId ? camp : c)))
          setCampForm((prev) => ({ ...prev, configVersion: cfg }))
          setAssignments(result.data.assignments || [])
          setAssignmentsDirty(false)
          notify.success("Campaign + assignments saved")
        } catch {
          notify.error("Save succeeded but configVersion missing - reloading")
          await reloadCampaigns()
          await loadGraph(editingCampId, { forceAssignments: true })
        }
      } else {
        const msg = (result.data as { message?: string } | undefined)?.message
        if (msg === "Conflict") {
          notify.error("Campaign was modified elsewhere - reloading")
          await reloadCampaigns()
          await loadGraph(editingCampId, { forceAssignments: true })
        } else {
          setError(result.error)
        }
      }
      return
    }

    const body: Record<string, unknown> = {
      name: campForm.name,
      advertiser: campForm.advertiser,
      startAt,
      endAt,
      weight: Number(campForm.weight),
      locale: campForm.locale || "all",
      enabled: campForm.enabled,
      packageId: campForm.packageId || undefined,
    }
    const clickUrl = versionForm.clickUrl.trim()
    const hasDraftVersion =
      !!clickUrl &&
      /^https?:\/\//i.test(clickUrl) &&
      (!!(versionForm.imageUrl || "").trim() || !!(versionForm.html || "").trim())
    if (assignments.length > 0 && !hasDraftVersion) {
      setBusy(false)
      setError({ errors: [{ message: "Assignments on create require a draft version (image and/or html + http(s) click URL)" }] })
      return
    }
    if (hasDraftVersion) {
      body.version = {
        imageUrl: versionForm.imageUrl.trim(),
        html: versionForm.html.trim(),
        clickUrl,
      }
      if (assignments.length > 0) {
        body.assignments = assignments.map((a) => ({
          placementId: a.placementId,
          creativeVersionId: 0,
        }))
      }
    }
    const result = await actions.createSponsorshipCampaign(body)
    setBusy(false)
    if (result.ok) {
      try {
        if (isCreateGraphResult(result.data)) {
          const camp = result.data.campaign
          const cfg = requireConfigVersion(result.data.configVersion ?? camp.configVersion)
          setCampaigns((prev) => [...prev, camp])
          setEditingCampId(camp.id)
          setCampForm({ ...campForm, configVersion: cfg })
          setVersions(result.data.versions || [])
          setAssignments(result.data.assignments || [])
          setAssignmentsDirty(false)
          setVersionForm({ imageUrl: "", html: "", clickUrl: "https://" })
          notify.success("Campaign created with assignments")
        } else {
          const camp = result.data as SponsorshipCampaign
          const cfg = requireConfigVersion(camp.configVersion)
          setCampaigns((prev) => [...prev, camp])
          setEditingCampId(camp.id)
          setCampForm({ ...campForm, configVersion: cfg })
          setAssignmentsDirty(false)
          await loadGraph(camp.id, { forceAssignments: true })
          notify.success("Campaign created - add a version and assignment, then Save to go live")
        }
      } catch {
        notify.error("Create succeeded but configVersion missing - reload the page")
      }
    } else {
      setError(result.error)
    }
  }

  const removeCampaign = async (id: number) => {
    if (!confirm("Delete this campaign?")) return
    const result = await actions.deleteSponsorshipCampaign(id)
    if (result.ok) {
      setCampaigns((prev) => prev.filter((c) => c.id !== id))
      if (editingCampId === id) {
        setEditingCampId(null)
        setCampForm(emptyCamp())
      }
      notify.success("Campaign deleted")
    }
  }

  const startEditCampaign = (c: SponsorshipCampaign) => {
    let cfg: number
    try {
      cfg = requireConfigVersion(c.configVersion)
    } catch {
      notify.error("Campaign missing configVersion from server")
      return
    }
    if (editingCampId !== c.id) {
      setAssignmentsDirty(false)
    }
    setEditingCampId(c.id)
    setCampForm({
      name: c.name,
      advertiser: c.advertiser || "",
      startAtLocal: utcToDatetimeLocalValue(c.startAt),
      endAtLocal: utcToDatetimeLocalValue(c.endAt),
      weight: c.weight,
      locale: c.locale || "all",
      enabled: c.enabled,
      packageId: c.packageId,
      configVersion: cfg,
    })
    setTab("campaigns")
  }

  const createVersion = async () => {
    if (!editingCampId) return
    const clickUrl = versionForm.clickUrl.trim()
    if (!/^https?:\/\//i.test(clickUrl)) {
      notify.error("clickUrl must be an http(s) URL")
      return
    }
    if (!(versionForm.imageUrl || "").trim() && !(versionForm.html || "").trim()) {
      notify.error("Provide imageUrl and/or html")
      return
    }
    let configVersion: number
    try {
      configVersion = requireConfigVersion(campForm.configVersion)
    } catch {
      notify.error("Missing configVersion - reload the campaign before creating a version")
      return
    }
    setBusy(true)
    try {
      const result = await actions.createCreativeVersion(editingCampId, {
        imageUrl: versionForm.imageUrl.trim(),
        html: versionForm.html.trim(),
        clickUrl,
        configVersion,
      })
      if (result.ok) {
        try {
          const cfg = requireConfigVersion(result.data.configVersion)
          setVersionForm({ imageUrl: "", html: "", clickUrl: "https://" })
          setCampForm((prev) => ({ ...prev, configVersion: cfg }))
          if (result.data.version) {
            setVersions((prev) => {
              const next = [result.data.version, ...prev.filter((v) => v.id !== result.data.version.id)]
              next.sort((a, b) => b.versionNo - a.versionNo)
              return next
            })
          }
          await reloadCampaigns()
          // Preserve staged assignment removals/adds (dirty guard).
          await loadGraph(editingCampId)
          notify.success(`Creative v${result.data.version.versionNo} created`)
        } catch {
          notify.error("Version created but configVersion missing - reloading")
          await reloadCampaigns()
          await loadGraph(editingCampId, { forceAssignments: true })
        }
      } else {
        const msg = (result.data as { message?: string } | undefined)?.message
        if (msg === "Conflict") {
          notify.error("Campaign was modified elsewhere - reloading")
          await reloadCampaigns()
          await loadGraph(editingCampId, { forceAssignments: true })
        } else {
          notify.error("Failed to create creative version")
          setError(result.error)
        }
      }
    } finally {
      setBusy(false)
    }
  }

  const upsertAssignment = () => {
    if (!assignPlacementId) return
    const creativeVersionId = editingCampId ? Number(assignVersionId) : 0
    if (editingCampId && (!assignVersionId || creativeVersionId <= 0)) return
    setAssignments((prev) => {
      const next = prev.filter((a) => a.placementId !== assignPlacementId)
      next.push({
        id: 0,
        campaignId: editingCampId || 0,
        placementId: assignPlacementId,
        creativeVersionId,
      })
      next.sort((a, b) => a.placementId.localeCompare(b.placementId))
      return next
    })
    setAssignmentsDirty(true)
    notify.success(editingCampId ? "Assignment staged - click Save campaign to persist" : "Assignment staged - included on Add campaign")
  }

  const removeAssignment = (placementId: string) => {
    if (!confirm(`Remove assignment for ${placementId}?`)) return
    setAssignments((prev) => prev.filter((a) => a.placementId !== placementId))
    setAssignmentsDirty(true)
    notify.success("Assignment removed from draft - click Save campaign to persist")
  }


  const savePlacement = async (p: AdPlacement) => {
    setBusy(true)
    setError(undefined)
    const result = await actions.updateAdPlacement(p.id, {
      adsenseSlotId: p.adsenseSlotId || "",
      adsenseFormat: p.adsenseFormat || "",
      emptyPolicy: p.emptyPolicy === "reserve" ? "reserve" : "collapse",
    })
    setBusy(false)
    if (result.ok && result.data) {
      setPlacements((prev) => prev.map((row) => (row.id === result.data!.id ? result.data! : row)))
      notify.success(`Placement ${p.id} saved`)
    } else {
      setError(result.error)
      notify.error("Could not save placement")
    }
  }

  const versionById = (id: number) => versions.find((v) => v.id === id)

  return (
    <VStack spacing={6}>
      <HStack spacing={2}>
        <Button variant={tab === "campaigns" ? "primary" : "tertiary"} onClick={() => setTab("campaigns")}>
          Campaigns
        </Button>
        <Button variant={tab === "packages" ? "primary" : "tertiary"} onClick={() => setTab("packages")}>
          Packages
        </Button>
        <Button variant={tab === "placements" ? "primary" : "tertiary"} onClick={() => setTab("placements")}>
          Placements
        </Button>
      </HStack>

      {tab === "packages" && (
        <VStack spacing={4}>
          <p className="text-muted text-sm">Packages on /advertise. Slot CSV is marketing-only (advisory placement ids). Selection never parses package slots.</p>
          <Form error={error}>
            <Input
              field="slug"
              label="Slug"
              value={pkgForm.slug}
              onChange={(v) => setPkgForm({ ...pkgForm, slug: v })}
              afterLabel={<FieldTip text="URL-ish id for this package on /advertise." />}
            />
            <Input
              field="name"
              label="Name"
              value={pkgForm.name}
              onChange={(v) => setPkgForm({ ...pkgForm, name: v })}
              afterLabel={<FieldTip text="Display name on /advertise. Not used for live ad selection." />}
            />
            <TextArea
              field="description"
              label="Description"
              value={pkgForm.description}
              onChange={(v) => setPkgForm({ ...pkgForm, description: v })}
              afterLabel={<FieldTip text="Copy on /advertise. Does not affect who wins a slot." />}
            />
            <Input
              field="slots"
              label="Slots CSV (marketing only)"
              value={pkgForm.slots}
              onChange={(v) => setPkgForm({ ...pkgForm, slots: v })}
              afterLabel={<FieldTip text={`Marketing copy only (e.g. ${slots.join(", ")}). Selection never parses this.`} />}
            />
            <Input
              field="durationDays"
              label="Duration days"
              value={String(pkgForm.durationDays)}
              onChange={(v) => setPkgForm({ ...pkgForm, durationDays: Number(v) || 0 })}
              afterLabel={<FieldTip text="Sold length shown on /advertise. Not the campaign schedule." />}
            />
            <Input
              field="sort"
              label="Sort"
              value={String(pkgForm.sort)}
              onChange={(v) => setPkgForm({ ...pkgForm, sort: Number(v) || 0 })}
              afterLabel={<FieldTip text="Display order on /advertise. Lower first." />}
            />
            <HStack spacing={2}>
              <Button variant="primary" onClick={savePackage} disabled={busy}>
                {editingPkgId ? "Update package" : "Add package"}
              </Button>
              {editingPkgId && (
                <Button variant="tertiary" onClick={() => { setEditingPkgId(null); setPkgForm(emptyPkg()) }}>
                  Cancel
                </Button>
              )}
            </HStack>
          </Form>
          <VStack spacing={2} divide>
            {packages.map((p) => (
              <HStack key={p.id} spacing={4} className="justify-between py-2">
                <div>
                  <div className="font-medium">{p.name} <span className="text-muted text-sm">({p.slug})</span></div>
                  <div className="text-muted text-sm">{p.slots} | {p.durationDays}d</div>
                </div>
                <HStack spacing={2}>
                  <Button variant="tertiary" onClick={() => { setEditingPkgId(p.id); setPkgForm({ slug: p.slug, name: p.name, description: p.description, slots: p.slots, durationDays: p.durationDays, sort: p.sort }) }}>Edit</Button>
                  <Button variant="danger" onClick={() => removePackage(p.id)}>Delete</Button>
                </HStack>
              </HStack>
            ))}
            {packages.length === 0 && <p className="text-muted">No packages yet.</p>}
          </VStack>
        </VStack>
      )}

      {tab === "campaigns" && (
        <VStack spacing={4}>
          <Hint permanentCloseKey="sponsorship-campaigns-howto">
            House ads overlay AdSense. A campaign is live only with a version assigned to a placement, enabled, in schedule, and a non-empty advertiser. First save can include draft version + placements; no assignments = not live. Dates are local ({tzLabel}), stored UTC.
            {editingCampId ? ` OCC config_version=${campForm.configVersion} — stale Save is 409, reload and retry.` : ""}
          </Hint>
          <Form error={error}>
            <Input
              field="name"
              label="Campaign name (internal)"
              value={campForm.name}
              onChange={(v) => setCampForm({ ...campForm, name: v })}
              afterLabel={<FieldTip text="Internal billing/reference. Not shown as the public Sponsored label." />}
            />
            <Input
              field="advertiser"
              label="Advertiser / company name"
              value={campForm.advertiser}
              onChange={(v) => setCampForm({ ...campForm, advertiser: v })}
              afterLabel={<FieldTip text="Shown as Sponsored - {name} outside the creative. Empty advertiser is never selected or rendered." />}
            />
            <Select
              key={`locale-${editingCampId ?? "new"}-${campForm.locale}`}
              field="locale"
              label="Locale"
              defaultValue={campForm.locale}
              options={localeOptions}
              onChange={(o) => o && setCampForm({ ...campForm, locale: o.value })}
              afterLabel={<FieldTip text="all = every locale. en/ru = that site language only." />}
            />
            <Input
              field="startAt"
              label={`Start (local ${tzLabel})`}
              type="datetime-local"
              value={campForm.startAtLocal}
              onChange={(v) => setCampForm({ ...campForm, startAtLocal: v })}
              afterLabel={<FieldTip text="Inclusive. Entered in your timezone, stored as UTC." />}
            />
            <Input
              field="endAt"
              label={`End (local ${tzLabel})`}
              type="datetime-local"
              value={campForm.endAtLocal}
              onChange={(v) => setCampForm({ ...campForm, endAtLocal: v })}
              afterLabel={<FieldTip text="Exclusive (now must be before this). Out of window: not selected, clicks 404." />}
            />
            <Input
              field="weight"
              label="Weight"
              value={String(campForm.weight)}
              onChange={(v) => setCampForm({ ...campForm, weight: Number(v) || 0 })}
              afterLabel={<FieldTip text="Share of the lottery vs other live campaigns in the same slot. 0 never wins." />}
            />
            <HStack spacing={1} className="mb-4 items-center">
              <Toggle field="enabled" label="Enabled" active={campForm.enabled} onToggle={(v) => setCampForm({ ...campForm, enabled: v })} />
              <FieldTip text="Off: not selected and clicks 404. Pause without deleting." />
            </HStack>

          <VStack spacing={4} className="p-4 border border-border rounded">
              <div className="font-medium">Assignments &amp; creative versions</div>
              <p className="text-xs text-muted">
                {editingCampId
                  ? "Editing creative = create a new version, then point the placement assignment at it. Old versions stay for click history (?v=)."
                  : "Optional on create: draft version (image and/or html + http(s) click URL) and at least one placement. That POST persists campaign + version + assignments in one transaction. No assignments = not live; selection will not fill."}
              </p>

              <div className="text-sm font-medium">{editingCampId ? "Create version" : "Draft version"}</div>
              <Input
                field="ver.imageUrl"
                label="Image URL"
                value={versionForm.imageUrl}
                onChange={(v) => setVersionForm({ ...versionForm, imageUrl: v })}
                afterLabel={<FieldTip text="Optional if HTML is set. Image-only ads may be a link." />}
              />
              <TextArea
                field="ver.html"
                label="HTML (sandboxed iframe - never innerHTML)"
                value={versionForm.html}
                onChange={(v) => setVersionForm({ ...versionForm, html: v })}
                afterLabel={<FieldTip text="Renders in a sandboxed iframe only, never inside a parent link. Optional if image is set." />}
              />
              <Input
                field="ver.clickUrl"
                label="Click URL"
                value={versionForm.clickUrl}
                onChange={(v) => setVersionForm({ ...versionForm, clickUrl: v })}
                afterLabel={<FieldTip text="Must be http(s) with no user:pass in the URL. Required for a version. Clicks go through /ads/click/:id?v=." />}
              />
              {editingCampId && (
                <Button variant="primary" onClick={createVersion} disabled={busy}>
                  Create immutable version
                </Button>
              )}

              {previewAd && (
                <div className="mb-2 p-3 border border-border rounded bg-elevated">
                  <div className="text-sm font-medium mb-2">Draft version preview</div>
                  {(() => {
                    const pl = placements.find((row) => row.id === previewAd.placementId)
                    const meta = pl
                      ? { kind: pl.kind, maxWidth: pl.maxWidth, maxHeight: pl.maxHeight, label: pl.name }
                      : undefined
                    if ((pl?.kind || "").toLowerCase() === "native") {
                      return <FeedNativeAd ad={previewAd} placement={meta} />
                    }
                    return (
                      <AdSlot
                        instanceId="admin-preview"
                        placementId={previewAd.placementId}
                        ad={previewAd}
                        allowAdSense={false}
                        placement={meta}
                      />
                    )
                  })()}
                </div>
              )}

              {editingCampId && (
                <>
              <div className="text-sm font-medium mt-2">Versions</div>
              <VStack spacing={1} divide>
                {versions.map((v) => (
                  <div key={v.id} className="text-sm py-1">
                    <strong>v{v.versionNo}</strong> (id {v.id}) | img={v.imageUrl ? "yes" : "no"} | html={v.html ? "yes" : "no"} | {v.clickUrl}
                  </div>
                ))}
                {versions.length === 0 && <p className="text-muted text-sm">No versions yet - create one above, then assign placements.</p>}
              </VStack>
                </>
              )}

              <div className="text-sm font-medium mt-2">Assign placement {"->"} version</div>
              <HStack spacing={2} className="flex-wrap items-end">
                <label className="text-sm">
                  Placement
                  <FieldTip text="Slot this campaign occupies (feed, sidebar, post, pages). Unassign + Save reveals AdSense if configured." />
                  <select
                    className="block mt-1 border border-border rounded px-2 py-1"
                    value={assignPlacementId}
                    onChange={(e) => setAssignPlacementId(e.target.value)}
                  >
                    <option value="">Select...</option>
                    {placements.filter((pl) => pl.enabled).map((pl) => (
                      <option key={pl.id} value={pl.id}>
                        {pl.name || SPONSORSHIP_SLOT_SPECS[pl.id]?.label || pl.id}
                        {pl.kind ? ` (${pl.kind})` : ""}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm">
                  Version
                  <FieldTip text="Which creative that slot uses. On create, the draft is saved with the campaign. Edit: new creative = new version, then re-assign." />
                  <select
                    className="block mt-1 border border-border rounded px-2 py-1"
                    value={assignVersionId === "" ? "" : String(assignVersionId)}
                    onChange={(e) => setAssignVersionId(e.target.value ? Number(e.target.value) : "")}
                  >
                    <option value="">Select...</option>
                    {editingCampId ? (
                      versions.map((v) => (
                        <option key={v.id} value={v.id}>v{v.versionNo} (#{v.id})</option>
                      ))
                    ) : (
                      <option value="0">Draft version (created on save)</option>
                    )}
                  </select>
                </label>
                <Button variant="primary" onClick={upsertAssignment} disabled={busy || !assignPlacementId || (editingCampId ? !assignVersionId : false)}>
                  Save assignment
                </Button>
              </HStack>

              <div className="text-sm font-medium mt-2">Current assignments</div>
              <VStack spacing={1} divide>
                {assignments.map((a) => {
                  const ver = versionById(a.creativeVersionId)
                  return (
                    <HStack key={a.placementId} spacing={4} className="justify-between py-1 text-sm">
                      <div>
                        <strong>{a.placementId}</strong> {"->"} {ver ? `v${ver.versionNo}` : a.creativeVersionId === 0 ? "draft version" : `version #${a.creativeVersionId}`}
                      </div>
                      <Button variant="danger" onClick={() => removeAssignment(a.placementId)}>Remove</Button>
                    </HStack>
                  )
                })}
                {assignments.length === 0 && <p className="text-muted text-sm">No assignments - selection will not fill placements for this campaign.</p>}
              </VStack>
            </VStack>

            <HStack spacing={2}>
              <Button variant="primary" onClick={saveCampaign} disabled={busy}>
                {editingCampId ? "Save campaign + assignments" : "Add campaign"}
              </Button>
              {editingCampId && (
                <Button variant="tertiary" onClick={() => { setEditingCampId(null); setCampForm(emptyCamp()) }}>
                  Cancel
                </Button>
              )}
            </HStack>
          </Form>

          <VStack spacing={2} divide>
            {campaigns.map((c) => {
              const status = deriveCampaignStatus(c)
              const badge = statusBadge(status)
              return (
                <HStack key={c.id} spacing={4} className="justify-between py-2">
                  <div>
                    <div className="font-medium flex items-center gap-2 flex-wrap">
                      {c.name}
                      <span className={`text-xs px-1.5 py-0.5 rounded ${badge.className}`}>{badge.label}</span>
                    </div>
                    <div className="text-muted text-sm">
                      {c.advertiser} | {c.locale} | clicks {c.clicks} | cfg v{c.configVersion}
                    </div>
                  </div>
                  <HStack spacing={2}>
                    <Button variant="tertiary" onClick={() => startEditCampaign(c)}>Edit</Button>
                    <Button variant="danger" onClick={() => removeCampaign(c.id)}>Delete</Button>
                  </HStack>
                </HStack>
              )
            })}
            {campaigns.length === 0 && <p className="text-muted">No campaigns yet.</p>}
          </VStack>
        </VStack>
      )}

      {tab === "placements" && (
        <VStack spacing={4}>
          <p className="text-muted text-sm">
            Catalog AdSense fallback per placement. Publisher id stays in GOOGLE_ADSENSE env; set unit slot ids / format / empty policy here (PUT /api/v1/ads/placements/:id).
          </p>
          <Form error={error}>
            <VStack spacing={4} divide>
              {placements.map((p) => (
                <VStack key={p.id} spacing={2} className="py-3">
                  <div className="font-medium">
                    {p.name || SPONSORSHIP_SLOT_SPECS[p.id]?.label || p.id}{" "}
                    <span className="text-muted text-sm">({p.id})</span>
                    {!p.enabled && <span className="text-xs ml-2 text-amber-700">disabled</span>}
                  </div>
                  <p className="text-xs text-muted">
                    kind: {p.kind || "frame"} (catalog; drives native vs frame renderer)
                    {p.maxWidth ? ` | max ${p.maxWidth}x${p.maxHeight || "?"}` : ""}
                  </p>
                  {!!p.description && <p className="text-xs text-muted">{p.description}</p>}
                   <Input
                     field={`adsenseSlotId.${p.id}`}
                     label="AdSense slot id"
                     value={p.adsenseSlotId || ""}
                     onChange={(v) =>
                       setPlacements((prev) => prev.map((row) => (row.id === p.id ? { ...row, adsenseSlotId: v } : row)))
                     }
                     afterLabel={<FieldTip text="Google ad unit id. Publisher id stays in GOOGLE_ADSENSE. Empty = no AdSense when house misses." />}
                   />
                   <Input
                     field={`adsenseFormat.${p.id}`}
                     label="AdSense format"
                     value={p.adsenseFormat || ""}
                     onChange={(v) =>
                       setPlacements((prev) => prev.map((row) => (row.id === p.id ? { ...row, adsenseFormat: v } : row)))
                     }
                     afterLabel={<FieldTip text="data-ad-format, e.g. auto, rectangle, fluid." />}
                   />
                   <label className="block text-sm mb-2">
                     Empty policy
                     <FieldTip text="If no house fill and no AdSense: collapse takes no space; reserve keeps the empty frame." />
                    <select
                      className="block mt-1 border border-border rounded px-2 py-1"
                      value={p.emptyPolicy === "reserve" ? "reserve" : "collapse"}
                      onChange={(e) =>
                        setPlacements((prev) =>
                          prev.map((row) =>
                            row.id === p.id
                              ? { ...row, emptyPolicy: e.target.value === "reserve" ? "reserve" : "collapse" }
                              : row
                          )
                        )
                      }
                    >
                      <option value="collapse">collapse</option>
                      <option value="reserve">reserve</option>
                    </select>
                  </label>
                  <Button
                    variant="primary"
                    onClick={() => {
                      const latest = placements.find((row) => row.id === p.id) || p
                      void savePlacement(latest)
                    }}
                    disabled={busy}
                  >
                    Save {p.id}
                  </Button>
                </VStack>
              ))}
              {placements.length === 0 && <p className="text-muted">No placements in catalog.</p>}
            </VStack>
          </Form>
        </VStack>
      )}
    </VStack>
  )
}

export default ManageSponsorshipPage
