import React from "react"
import { SponsorImageCrop } from "@fider/models/sponsorBooking"

export const defaultBannerCrop: SponsorImageCrop = { x: 50, y: 50, zoom: 1 }

export function SponsorBannerImage({ imageKey, alt, device, crop = defaultBannerCrop, imageRef, onLoad, onError }: {
  imageKey: string
  alt: string
  device: string
  crop?: SponsorImageCrop
  imageRef?: React.Ref<HTMLImageElement>
  onLoad?: () => void
  onError?: () => void
}) {
  const position = `${crop.x}% ${crop.y}%`
  const imageURL = `/static/images/${imageKey}`

  return (
    <div className="overflow-hidden" style={{ height: device === "mobile" ? 80 : 128 }}>
      <picture>
        <source media="(min-width: 768px), (min-resolution: 2dppx)" srcSet={`${imageURL}?size=1500`} />
        <img
          ref={imageRef}
          src={`${imageURL}?size=512`}
          alt={alt}
          className="pointer-events-none block h-full w-full select-none object-cover"
          style={{ objectPosition: position, transformOrigin: position, transform: `scale(${crop.zoom})` }}
          onLoad={onLoad}
          onError={onError}
          draggable={false}
        />
      </picture>
    </div>
  )
}
