import React, { useState } from "react"
import { Button, ImageUploader, Input, TextArea } from "@fider/components"
import { ImageUpload } from "@fider/models"
import { SponsorCreative } from "@fider/models/sponsorBooking"
import { utcToDatetimeLocalValue } from "@fider/components/sponsorship/datetime"
import { uploadSponsorImage } from "@fider/services/actions/sponsorBooking"
import { newFileUploadID } from "@fider/services/actions/file"
import { controlClass } from "./SponsorControls"
import { useSponsorSave } from "./useSponsorSave"
import { BannerCropper } from "./BannerCropper"

const utmFields = [
  { key: "utm_source", label: "Source" },
  { key: "utm_medium", label: "Medium" },
  { key: "utm_campaign", label: "Campaign" },
  { key: "utm_term", label: "Term" },
  { key: "utm_content", label: "Content" },
]

export function newSponsorCreative(campaignId: number): SponsorCreative {
  return {
    id: 0,
    revision: 0,
    campaignId,
    state: "draft",
    reviewReason: "",
    framed: false,
    language: "",
    device: "",
    startAt: null,
    endAt: null,
    headline: "",
    description: "",
    logoKey: "",
    imageKey: "",
    callToAction: "",
    destination: "",
    offerCode: "",
    offerTerms: "",
    offerExpires: null,
  }
}

function CreativeImage({ label, value, disabled, onChanged, onPending }: {
  label: string
  value: string
  disabled: boolean
  onChanged: (key: string) => void
  onPending: (pending: boolean) => void
}) {
  const [file, setFile] = useState<ImageUpload>()
  const upload = useSponsorSave(uploadSponsorImage, result => {
    onChanged(result.blobKey)
    onPending(false)
    setFile(undefined)
  }, newFileUploadID)

  const changed = (image: ImageUpload) => {
    if (image.remove) {
      onChanged("")
      onPending(false)
      setFile(undefined)
      return
    }

    setFile(image)
    void upload.submit(image)
  }

  return (
    <div>
      <ImageUploader
        key={value}
        field={label.toLowerCase()}
        label={label}
        bkey={value}
        disabled={disabled || upload.disabled}
        onRead={() => onPending(true)}
        onChange={changed}
      />
      {file && upload.error && (
        <Button onClick={() => upload.uncertain ? upload.retry() : upload.submit(file)} loading={upload.busy}>
          Retry image upload
        </Button>
      )}
    </div>
  )
}

interface CreativeFieldsProps {
  value: SponsorCreative
  disabled: boolean
  imagePending: boolean
  onChange: React.Dispatch<React.SetStateAction<SponsorCreative>>
  onPendingImage: (field: "image" | "logo", pending: boolean) => void
}

export function CreativeFields({ value, disabled, imagePending, onChange, onPendingImage }: CreativeFieldsProps) {
  const [cropping, setCropping] = useState(false)
  let destination: URL | undefined
  try {
    destination = new URL(value.destination)
  } catch {
    destination = undefined
  }

  const change = <K extends keyof SponsorCreative>(key: K, fieldValue: SponsorCreative[K]) => {
    onChange(previous => ({ ...previous, [key]: fieldValue }))
  }

  const setDate = (key: "startAt" | "endAt" | "offerExpires", input: string) => {
    const date = input ? new Date(input) : null
    if (!date || Number.isFinite(date.getTime())) {
      change(key, date?.toISOString() ?? null)
    }
  }

  return (
    <>
      <fieldset disabled={disabled} className="min-w-0 space-y-4">
        <CreativeImage
          label="Image"
          value={value.imageKey}
          disabled={disabled}
          onChanged={key => onChange(previous => ({ ...previous, imageKey: key, bannerCrop: undefined }))}
          onPending={pending => onPendingImage("image", pending)}
        />
        {value.imageKey && <Button disabled={imagePending} onClick={() => setCropping(true)}>Crop banner</Button>}
        {cropping && (
          <BannerCropper
            imageKey={value.imageKey}
            crop={value.bannerCrop}
            onApply={crop => change("bannerCrop", crop)}
            onClose={() => setCropping(false)}
          />
        )}
        <Input field="destination" label="Destination URL" value={value.destination} onChange={text => change("destination", text)} />
        <details>
          <summary className="cursor-pointer font-medium">Text and styling</summary>
          <div className="mt-4 space-y-4">
            <label className="flex cursor-pointer items-center gap-2 text-sm">
              <input type="checkbox" className="cursor-pointer accent-primary" checked={value.framed} onChange={event => change("framed", event.target.checked)} />
              Frame
            </label>
            <Input field="headline" label="Headline" value={value.headline} onChange={text => change("headline", text)} />
            <TextArea field="description" label="Description" value={value.description} onChange={text => change("description", text)} />
            <CreativeImage
              label="Logo"
              value={value.logoKey}
              disabled={disabled}
              onChanged={key => change("logoKey", key)}
              onPending={pending => onPendingImage("logo", pending)}
            />
            <Input field="callToAction" label="Button label" value={value.callToAction} onChange={text => change("callToAction", text)} />
          </div>
        </details>
        <details>
          <summary className="cursor-pointer font-medium">UTM tags</summary>
          <div className="mt-4 grid gap-x-5 sm:grid-cols-2">
            {utmFields.map(field => (
              <Input
                key={field.key}
                field={field.key}
                label={field.label}
                disabled={!destination}
                value={destination?.searchParams.get(field.key) ?? ""}
                onChange={text => onChange(previous => {
                  const updated = new URL(previous.destination)
                  if (text) {
                    updated.searchParams.set(field.key, text)
                  } else {
                    updated.searchParams.delete(field.key)
                  }

                  return { ...previous, destination: updated.toString() }
                })}
              />
            ))}
          </div>
        </details>
        <details>
          <summary className="cursor-pointer font-medium">Creative schedule and audience</summary>
          <div className="mt-4 grid gap-5 sm:grid-cols-2">
            <label className="text-sm font-medium">
              Language
              <select className={controlClass} value={value.language} onChange={event => change("language", event.target.value)}>
                <option value="">All languages</option>
                <option value="en">English</option>
                <option value="ru">Russian</option>
              </select>
            </label>
            <label className="text-sm font-medium">
              Device
              <select className={controlClass} value={value.device} onChange={event => change("device", event.target.value)}>
                <option value="">All devices</option>
                <option value="desktop">Desktop</option>
                <option value="mobile">Mobile</option>
              </select>
            </label>
            <label className="text-sm font-medium">
              Start (empty for booking start)
              <input type="datetime-local" className={controlClass} value={value.startAt ? utcToDatetimeLocalValue(value.startAt) : ""} onChange={event => setDate("startAt", event.target.value)} />
            </label>
            <label className="text-sm font-medium">
              End (empty for booking end)
              <input type="datetime-local" className={controlClass} value={value.endAt ? utcToDatetimeLocalValue(value.endAt) : ""} onChange={event => setDate("endAt", event.target.value)} />
            </label>
          </div>
        </details>
        <details>
          <summary className="cursor-pointer font-medium">Offer</summary>
          <div className="mt-4 grid gap-5 sm:grid-cols-2">
            <Input field="offerCode" label="Discount code" value={value.offerCode} onChange={text => change("offerCode", text)} />
            <label className="text-sm font-medium">
              Offer expiry
              <input type="datetime-local" className={controlClass} value={value.offerExpires ? utcToDatetimeLocalValue(value.offerExpires) : ""} onChange={event => setDate("offerExpires", event.target.value)} />
            </label>
          </div>
          <TextArea field="offerTerms" label="Eligibility and terms" value={value.offerTerms} onChange={text => change("offerTerms", text)} />
        </details>
      </fieldset>
      <fieldset disabled={disabled} className="mt-5 grid gap-5 sm:grid-cols-2">
        <label className="text-sm font-medium">
          Review status
          <select className={controlClass} value={value.state} onChange={event => change("state", event.target.value as SponsorCreative["state"])}>
            <option value="draft">Draft</option>
            <option value="review">Awaiting review</option>
            <option value="approved">Approved</option>
            <option value="rejected">Rejected</option>
          </select>
        </label>
        <Input field="reviewReason" label="Review reason" value={value.reviewReason} onChange={text => change("reviewReason", text)} />
      </fieldset>
    </>
  )
}
