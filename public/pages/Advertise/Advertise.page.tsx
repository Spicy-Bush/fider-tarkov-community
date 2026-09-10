import React from "react"
import { Button } from "@fider/components"
import { SponsorshipPackage } from "@fider/models"
import { VStack } from "@fider/components/layout"
import { heroiconsSpeakerphone as IconSpeaker } from "@fider/icons.generated"
import { Icon } from "@fider/components"

interface AdvertisePageProps {
  packages: SponsorshipPackage[]
  contact: string
}

const AdvertisePage: React.FC<AdvertisePageProps> = (props) => {
  const packages = props.packages || []
  const contact = props.contact || "contact@tarkov.community"
  const mailto = `mailto:${contact}?subject=${encodeURIComponent("Sponsorship inquiry - tarkov.community")}`

  return (
    <div id="p-advertise" className="page container w-max-4xl py-10 pb-24">
      <VStack spacing={6}>
        <div className="flex items-center gap-3">
          <Icon sprite={IconSpeaker} className="h-8 w-8 text-primary" />
          <h1 className="text-2xl font-bold tracking-tight">Advertise</h1>
        </div>
        <p className="text-muted text-base leading-relaxed">
          Reach Escape from Tarkov players on tarkov.community with native feed placements and
          contextual slots. Packages below describe what we offer - no public pricing. Tell us
          your goals and we will follow up with availability and rates.
        </p>

        <div className="grid gap-4 sm:grid-cols-2">
          {packages.length === 0 ? (
            <p className="text-muted col-span-full">
              Packages will appear here once configured. You can still reach out to discuss a custom placement.
            </p>
          ) : (
            packages.map((pkg) => (
              <div key={pkg.id} className="rounded-panel border border-border bg-elevated p-4">
                <h2 className="text-lg font-semibold mb-1">{pkg.name}</h2>
                {pkg.description ? <p className="text-muted text-sm mb-3 whitespace-pre-wrap">{pkg.description}</p> : null}
                <ul className="text-sm text-muted space-y-1">
                  {pkg.slots ? <li>Slots: {pkg.slots}</li> : null}
                  {pkg.durationDays > 0 ? <li>Typical duration: {pkg.durationDays} days</li> : null}
                </ul>
              </div>
            ))
          )}
        </div>

        <div className="rounded-panel border border-primary/30 bg-accent-light/40 p-5">
          <h2 className="text-lg font-semibold mb-2">Get in touch</h2>
          <p className="text-muted text-sm mb-4">
            Email us with your brand, preferred dates, and creative format (image URL or HTML). We do not list dollar amounts on this page.
          </p>
          <Button variant="primary" href={mailto}>
            Contact {contact}
          </Button>
        </div>
      </VStack>
    </div>
  )
}

export default AdvertisePage
