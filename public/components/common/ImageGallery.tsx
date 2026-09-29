import React, { useState, useEffect, useCallback } from "react"
import { createPortal } from "react-dom"
import { uploadedImageURL, classSet } from "@fider/services/utils"
import { Icon } from "@fider/components/common/Icon"
import { useBodyScrollLock } from "@fider/hooks/useBodyScrollLock"
import { heroiconsChevronUp as IconChevron, heroiconsX as IconClose } from "@fider/icons.generated"
import { DraftImage } from "@fider/services/draftImages"

type ImageGalleryProps = { bkeys: string[] } | { uploads: DraftImage[] }

export const ImageGallery: React.FC<ImageGalleryProps> = (props) => {
  const images = "uploads" in props
    ? props.uploads.filter((image) => image.kind === "stored" || image.kind === "local")
    : props.bkeys

  const [showModal, setShowModal] = useState(false)
  const [currentIndex, setCurrentIndex] = useState(0)
  const [loadedSource, setLoadedSource] = useState<string>()
  const [fileURLs, setFileURLs] = useState<Map<File, string>>(new Map())

  const uploads = "uploads" in props ? props.uploads : undefined
  useEffect(() => {
    const urls = new Map<File, string>()
    for (const image of uploads || []) {
      if (image.kind === "local") urls.set(image.file, URL.createObjectURL(image.file))
    }
    setFileURLs(urls)
    return () => { for (const url of urls.values()) URL.revokeObjectURL(url) }
  }, [uploads])

  useBodyScrollLock(showModal)

  const imageURL = (image: string | Extract<DraftImage, { kind: "local" | "stored" }>, size: number) => {
    if (typeof image !== "string" && image.kind === "local") return fileURLs.get(image.file)

    return uploadedImageURL(typeof image === "string" ? image : image.bkey, size)
  }

  const openModal = (index: number) => {
    setCurrentIndex(index)
    setLoadedSource(undefined)
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
  }

  const goToNext = useCallback(() => {
    setCurrentIndex((prev) => (prev + 1) % images.length)
  }, [images.length])

  const goToPrevious = useCallback(() => {
    setCurrentIndex((prev) => (prev - 1 + images.length) % images.length)
  }, [images.length])

  useEffect(() => {
    if (!showModal) {
      return
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "ArrowLeft") goToPrevious()
      if (e.key === "ArrowRight") goToNext()
      if (e.key === "Escape") closeModal()
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [showModal, goToNext, goToPrevious])

  if (images.length === 0) {
    return null
  }

  const isSingleImage = images.length === 1
  const currentSource = showModal ? imageURL(images[currentIndex], 1500) : undefined
  const loaded = loadedSource === currentSource

  return (
    <>
      <div className={classSet({ "mt-2 w-full": true, "max-w-[280px]": isSingleImage })}>
        {isSingleImage ? (
          <div 
            className="relative cursor-pointer overflow-hidden rounded-card"
            onClick={() => openModal(0)}
          >
            <img 
              src={imageURL(images[0], 200)}
              alt="" 
              loading="lazy"
              className="w-full h-auto max-h-[180px] object-cover transition-transform duration-75 hover:scale-[1.02]"
            />
          </div>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {images.slice(0, 4).map((image, index) => (
              <div 
                key={index}
                className="relative cursor-pointer overflow-hidden rounded-card w-[80px] h-[80px]"
                onClick={() => openModal(index)}
              >
                <img 
                  src={imageURL(image, 200)}
                  alt="" 
                  loading="lazy"
                  className="w-full h-full object-cover transition-transform duration-75 hover:scale-105"
                />
                {images.length > 4 && index === 3 && (
                  <div className="absolute inset-0 bg-black/60 text-white flex items-center justify-center text-lg font-bold">
                    +{images.length - 4}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {showModal && createPortal(
        <div 
          className="fixed inset-0 z-10000 bg-black/90 flex items-center justify-center"
          onClick={closeModal}
        >
          <button
            className="absolute top-4 right-4 z-10 p-2 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer border-0 transition-colors"
            onClick={closeModal}
          >
            <Icon sprite={IconClose} className="w-6 h-6" />
          </button>

          {images.length > 1 && (
            <>
              <button
                className="absolute left-4 top-1/2 -translate-y-1/2 z-10 p-2 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer border-0 transition-colors"
                onClick={(e) => { e.stopPropagation(); goToPrevious(); }}
              >
                <Icon sprite={IconChevron} className="w-6 h-6 -rotate-90" />
              </button>
              <button
                className="absolute right-4 top-1/2 -translate-y-1/2 z-10 p-2 rounded-full bg-white/10 hover:bg-white/20 text-white cursor-pointer border-0 transition-colors"
                onClick={(e) => { e.stopPropagation(); goToNext(); }}
              >
                <Icon sprite={IconChevron} className="w-6 h-6 rotate-90" />
              </button>
            </>
          )}

          <div 
            className="flex items-center justify-center max-w-[90vw] max-h-[90vh]"
            onClick={(e) => e.stopPropagation()}
          >
            {!loaded && (
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="w-8 h-8 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              </div>
            )}
            <img 
              src={currentSource}
              alt=""
              onLoad={() => setLoadedSource(currentSource)}
              className={classSet({
                "max-w-full max-h-[85vh] object-contain rounded-panel transition-opacity duration-75": true,
                "opacity-0": !loaded,
                "opacity-100": loaded,
              })}
            />
          </div>

          {images.length > 1 && (
            <div className="absolute bottom-4 left-1/2 -translate-x-1/2 text-white/80 text-sm font-medium">
              {currentIndex + 1} / {images.length}
            </div>
          )}
        </div>,
        document.body
      )}
    </>
  )
}
