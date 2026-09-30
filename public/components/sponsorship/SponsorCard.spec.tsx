import React from "react"
import { render, screen } from "@testing-library/react"
import { SponsorSelection } from "@fider/models/sponsorBooking"
import { SponsorCard } from "./SponsorCard"

test("an image-only creative has one disclosure and keeps its destination intact", () => {
  const destination = "https://example.com/offer?sku=42&utm_source=tarkov.community&utm_campaign=launch%20week#details"
  const selection: SponsorSelection = {
    kind: "sponsor",
    clickUrl: "/sponsorship/click?token=signed-delivery",
    advertiser: "Example sponsor",
    expiresAt: "2999-01-01T00:00:00Z",
    placement: {
      id: "strip_desktop", name: "Community banner", pageType: "all",
      device: "desktop", enabled: true, position: "navigation", every: 0, empty: "none",
    },
    creative: {
      framed: false, headline: "", description: "", logoKey: "", imageKey: "attachments/banner.webp",
      callToAction: "", destination, offerCode: "", offerTerms: "", offerExpires: null,
    },
  }

  const view = render(<SponsorCard selection={selection} />)
  expect(screen.getByRole("complementary")).toHaveTextContent(/^Advertisement$/)
  expect(screen.getByRole("link", { name: "Example sponsor" })).toHaveAttribute("href", selection.clickUrl)
  expect(screen.getByRole("img")).toHaveAttribute("alt", "Example sponsor")

  view.rerender(<SponsorCard selection={selection} preview />)
  expect(screen.queryByRole("link")).not.toBeInTheDocument()

  view.rerender(<SponsorCard selection={{ ...selection, expiresAt: "2000-01-01T00:00:00Z" }} />)
  expect(screen.queryByRole("complementary")).not.toBeInTheDocument()
})
