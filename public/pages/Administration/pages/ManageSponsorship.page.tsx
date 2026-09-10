import React, { useMemo, useState } from "react"
import { Button, Form, Input, TextArea, Toggle, Select, SelectOption } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"
import { SponsorshipCampaign, SponsorshipPackage, SPONSORSHIP_SLOTS } from "@fider/models"
import { actions, Failure, notify } from "@fider/services"

export const pageConfig: PageConfig = {
  title: "Sponsorship",
  subtitle: "Packages and house ad campaigns (no public prices)",
  sidebarItem: "sponsorship",
}

interface ManageSponsorshipPageProps {
  campaigns: SponsorshipCampaign[]
  packages: SponsorshipPackage[]
  slots: string[]
}

type Tab = "campaigns" | "packages"

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
    slotId: "feed_native",
    creativeImageUrl: "",
    creativeHtml: "",
    clickUrl: "https://",
    startAt: start.toISOString().slice(0, 16),
    endAt: end.toISOString().slice(0, 16),
    weight: 100,
    locale: "all",
    enabled: true,
    packageId: undefined as number | undefined,
  }
}

const toIso = (localDatetime: string) => {
  // datetime-local -> ISO; append Z-less local as Date
  const d = new Date(localDatetime)
  return d.toISOString()
}

const ManageSponsorshipPage: React.FC<ManageSponsorshipPageProps> = (props) => {
  const slots = props.slots?.length ? props.slots : [...SPONSORSHIP_SLOTS]
  const [tab, setTab] = useState<Tab>("campaigns")
  const [packages, setPackages] = useState<SponsorshipPackage[]>(props.packages || [])
  const [campaigns, setCampaigns] = useState<SponsorshipCampaign[]>(props.campaigns || [])
  const [pkgForm, setPkgForm] = useState(emptyPkg())
  const [editingPkgId, setEditingPkgId] = useState<number | null>(null)
  const [campForm, setCampForm] = useState(emptyCamp())
  const [editingCampId, setEditingCampId] = useState<number | null>(null)
  const [error, setError] = useState<Failure | undefined>()
  const [busy, setBusy] = useState(false)

  const slotOptions: SelectOption[] = useMemo(
    () => slots.map((s) => ({ value: s, label: s })),
    [slots]
  )
  const localeOptions: SelectOption[] = [
    { value: "all", label: "all" },
    { value: "en", label: "en" },
    { value: "ru", label: "ru" },
  ]

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

  const saveCampaign = async () => {
    setBusy(true)
    setError(undefined)
    const body = {
      name: campForm.name,
      slotId: campForm.slotId,
      creativeImageUrl: campForm.creativeImageUrl,
      creativeHtml: campForm.creativeHtml,
      clickUrl: campForm.clickUrl,
      startAt: toIso(campForm.startAt),
      endAt: toIso(campForm.endAt),
      weight: Number(campForm.weight),
      locale: campForm.locale || "all",
      enabled: campForm.enabled,
      packageId: campForm.packageId || undefined,
    }
    const result = editingCampId
      ? await actions.updateSponsorshipCampaign(editingCampId, body)
      : await actions.createSponsorshipCampaign(body)
    setBusy(false)
    if (result.ok) {
      if (editingCampId) {
        setCampaigns((prev) => prev.map((c) => (c.id === editingCampId ? result.data : c)))
      } else {
        setCampaigns((prev) => [...prev, result.data])
      }
      setCampForm(emptyCamp())
      setEditingCampId(null)
      notify.success("Campaign saved")
    } else {
      setError(result.error)
    }
  }

  const removeCampaign = async (id: number) => {
    if (!confirm("Delete this campaign?")) return
    const result = await actions.deleteSponsorshipCampaign(id)
    if (result.ok) {
      setCampaigns((prev) => prev.filter((c) => c.id !== id))
      notify.success("Campaign deleted")
    }
  }

  const startEditCampaign = (c: SponsorshipCampaign) => {
    setEditingCampId(c.id)
    setCampForm({
      name: c.name,
      slotId: c.slotId,
      creativeImageUrl: c.creativeImageUrl || "",
      creativeHtml: c.creativeHtml || "",
      clickUrl: c.clickUrl,
      startAt: new Date(c.startAt).toISOString().slice(0, 16),
      endAt: new Date(c.endAt).toISOString().slice(0, 16),
      weight: c.weight,
      locale: c.locale || "all",
      enabled: c.enabled,
      packageId: c.packageId,
    })
    setTab("campaigns")
  }

  return (
    <VStack spacing={6}>
      <HStack spacing={2}>
        <Button variant={tab === "campaigns" ? "primary" : "tertiary"} onClick={() => setTab("campaigns")}>
          Campaigns
        </Button>
        <Button variant={tab === "packages" ? "primary" : "tertiary"} onClick={() => setTab("packages")}>
          Packages
        </Button>
      </HStack>

      {tab === "packages" && (
        <VStack spacing={4}>
          <p className="text-muted text-sm">Sellable packages shown on /advertise. Do not put dollar amounts in descriptions if you want them private.</p>
          <Form error={error}>
            <Input field="slug" label="Slug" value={pkgForm.slug} onChange={(v) => setPkgForm({ ...pkgForm, slug: v })} />
            <Input field="name" label="Name" value={pkgForm.name} onChange={(v) => setPkgForm({ ...pkgForm, name: v })} />
            <TextArea field="description" label="Description" value={pkgForm.description} onChange={(v) => setPkgForm({ ...pkgForm, description: v })} />
            <Input field="slots" label="Slots (comma-separated)" value={pkgForm.slots} onChange={(v) => setPkgForm({ ...pkgForm, slots: v })} />
            <Input field="durationDays" label="Duration days" value={String(pkgForm.durationDays)} onChange={(v) => setPkgForm({ ...pkgForm, durationDays: Number(v) || 0 })} />
            <Input field="sort" label="Sort" value={String(pkgForm.sort)} onChange={(v) => setPkgForm({ ...pkgForm, sort: Number(v) || 0 })} />
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
                  <div className="text-muted text-sm">{p.slots} · {p.durationDays}d</div>
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
          <p className="text-muted text-sm">House ads fill slots when enabled and within start/end. Fallback is AdSense (if configured) then empty.</p>
          <Form error={error}>
            <Input field="name" label="Name" value={campForm.name} onChange={(v) => setCampForm({ ...campForm, name: v })} />
            <Select field="slotId" label="Slot" defaultValue={campForm.slotId} options={slotOptions} onChange={(o) => o && setCampForm({ ...campForm, slotId: o.value })} />
            <Select field="locale" label="Locale" defaultValue={campForm.locale} options={localeOptions} onChange={(o) => o && setCampForm({ ...campForm, locale: o.value })} />
            <Input field="creativeImageUrl" label="Creative image URL" value={campForm.creativeImageUrl} onChange={(v) => setCampForm({ ...campForm, creativeImageUrl: v })} />
            <TextArea field="creativeHtml" label="Creative HTML (optional)" value={campForm.creativeHtml} onChange={(v) => setCampForm({ ...campForm, creativeHtml: v })} />
            <Input field="clickUrl" label="Click URL" value={campForm.clickUrl} onChange={(v) => setCampForm({ ...campForm, clickUrl: v })} />
            <Input field="startAt" label="Start (local)" type="datetime-local" value={campForm.startAt} onChange={(v) => setCampForm({ ...campForm, startAt: v })} />
            <Input field="endAt" label="End (local)" type="datetime-local" value={campForm.endAt} onChange={(v) => setCampForm({ ...campForm, endAt: v })} />
            <Input field="weight" label="Weight" value={String(campForm.weight)} onChange={(v) => setCampForm({ ...campForm, weight: Number(v) || 0 })} />
            <Toggle field="enabled" label="Enabled" active={campForm.enabled} onToggle={(v) => setCampForm({ ...campForm, enabled: v })} />
            <HStack spacing={2}>
              <Button variant="primary" onClick={saveCampaign} disabled={busy}>
                {editingCampId ? "Update campaign" : "Add campaign"}
              </Button>
              {editingCampId && (
                <Button variant="tertiary" onClick={() => { setEditingCampId(null); setCampForm(emptyCamp()) }}>
                  Cancel
                </Button>
              )}
            </HStack>
          </Form>
          <VStack spacing={2} divide>
            {campaigns.map((c) => (
              <HStack key={c.id} spacing={4} className="justify-between py-2">
                <div className="min-w-0">
                  <div className="font-medium truncate">{c.name}</div>
                  <div className="text-muted text-sm">
                    {c.slotId} · {c.locale} · {c.enabled ? "on" : "off"} · clicks {c.clicks}
                  </div>
                </div>
                <HStack spacing={2}>
                  <Button variant="tertiary" onClick={() => startEditCampaign(c)}>Edit</Button>
                  <Button variant="danger" onClick={() => removeCampaign(c.id)}>Delete</Button>
                </HStack>
              </HStack>
            ))}
            {campaigns.length === 0 && <p className="text-muted">No campaigns yet.</p>}
          </VStack>
        </VStack>
      )}
    </VStack>
  )
}

export default ManageSponsorshipPage
