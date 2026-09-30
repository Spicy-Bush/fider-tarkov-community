import React from "react"
import { Button } from "@fider/components"
import { SponsorshipPackage } from "@fider/models"
import { SponsorPlacement } from "@fider/models/sponsorBooking"

interface AdvertisePageProps {
  packages: SponsorshipPackage[]
  placements: SponsorPlacement[]
  contact: string
}

export default function AdvertisePage({ packages, placements, contact }: AdvertisePageProps) {
  const subject = encodeURIComponent("Sponsorship enquiry - tarkov.community")
  const mailto = `mailto:${contact}?subject=${subject}`

  return (
    <div id="p-advertise" className="mx-auto w-full max-w-4xl px-4 py-10 sm:px-6 sm:py-16">
      <header className="grid max-w-2xl gap-4">
        <h1 className="text-large">Sponsor Tarkov Community</h1>
        <p className="m-0 text-lg leading-relaxed text-muted">
          Reach Tarkov players and help keep this community page running!
        </p>
      </header>

      <section aria-label="Placements" className="my-10 grid gap-8 border-y border-border py-8 sm:grid-cols-2 sm:gap-12">
        <article className="flex flex-col gap-2">
          <div aria-hidden="true" className="mb-5 flex h-24 items-start rounded border border-border p-3">
            <div className="h-7 w-full rounded-sm bg-primary/25" />
          </div>
          <h2 className="text-title">Community banner</h2>
          <p className="m-0 leading-relaxed text-muted">
            A wide image beneath the navigation.
          </p>
        </article>
        <article className="flex flex-col gap-2">
          <div aria-hidden="true" className="mb-5 flex h-24 items-end gap-3 rounded border border-border p-3">
            <div className="h-full w-1/3 rounded-sm bg-primary/25" />
            <div className="h-7 flex-1 rounded-sm bg-primary/25" />
          </div>
          <h2 className="text-title">Sponsor panels</h2>
          <p className="m-0 leading-relaxed text-muted">
            Image placements beside the home feed or on post and community Pages.
          </p>
        </article>
      </section>

      {packages.length > 0 && (
        <section aria-labelledby="sponsor-packages" className="mb-10 grid gap-5">
          <h2 id="sponsor-packages" className="text-display">Packages</h2>
          <div className="divide-y divide-border">
            {packages.map(item => {
              const slotIDs = item.slots.split(",")
              const included = placements.filter(placement => slotIDs.includes(placement.id))

              return (
                <article key={item.id} className="py-5 first:pt-0">
                  <div className="flex flex-wrap items-baseline justify-between gap-2">
                    <h3 className="text-title">{item.name}</h3>
                    <span className="text-sm text-muted">{item.durationDays} days</span>
                  </div>
                  {item.description && <p className="mb-0 mt-2 whitespace-pre-line text-muted">{item.description}</p>}
                  {included.length > 0 && (
                    <p className="mb-0 mt-2 text-sm text-muted">{included.map(placement => placement.name).join(", ")}</p>
                  )}
                </article>
              )
            })}
          </div>
        </section>
      )}

      <section className="flex max-w-2xl flex-col items-start gap-3">
        <h2 className="text-display">Plan a campaign</h2>
        <p className="mb-5 leading-relaxed text-muted">
          Send your brand, preferred dates and placements to {contact}.
          We will confirm availability, rates and artwork.
        </p>
        <Button variant="primary" href={mailto}>Enquire about sponsorship</Button>
      </section>
    </div>
  )
}