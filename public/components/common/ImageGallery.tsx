import React, { useState, useEffect, useCallback, useRef } from "react"
import { uploadedImageURL } from "@fider/services"
import { Modal, Button, Loader } from "@fider/components"
import "./ImageGallery.scss"
import { Trans } from "@lingui/react/macro"

interface ImageGalleryProps {
  bkeys: string[]
}

export const ImageGallery: React.FC<ImageGalleryProps> = ({ bkeys }) => {
  const [showModal, setShowModal] = useState(false)
  const [currentIndex, setCurrentIndex] = useState(0)
  const [loadedPreview, setLoadedPreview] = useState(false)
  const [touchStart, setTouchStart] = useState<number | null>(null)
  const [touchEnd, setTouchEnd] = useState<number | null>(null)
  const [touchStartY, setTouchStartY] = useState<number | null>(null)
  const [touchEndY, setTouchEndY] = useState<number | null>(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [scale, setScale] = useState(1)
  const [isPinching, setIsPinching] = useState(false)
  const [lastDistance, setLastDistance] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  const openModal = (index: number) => {
    setCurrentIndex(index)
    setLoadedPreview(false)
    setShowModal(true)
    setScale(1)
  }

  const closeModal = () => {
    if (isFullscreen) {
      exitFullscreen()
    }
    setShowModal(false)
    setScale(1)
  }

  const goToNext = useCallback(() => {
    setLoadedPreview(false)
    setScale(1)
    setCurrentIndex((prev) => (prev + 1) % bkeys.length)
  }, [bkeys.length])

  const goToPrevious = useCallback(() => {
    setLoadedPreview(false)
    setScale(1)
    setCurrentIndex((prev) => (prev - 1 + bkeys.length) % bkeys.length)
  }, [bkeys.length])

  const onPreviewLoad = () => {
    setLoadedPreview(true)
  }

  const toggleFullscreen = () => {
    if (isFullscreen) {
      exitFullscreen()
    } else {
      enterFullscreen()
    }
  }

  const enterFullscreen = () => {
    const element = containerRef.current
    if (element) {
      if (element.requestFullscreen) {
        element.requestFullscreen()
      } else if ((element as any).webkitRequestFullscreen) {
        (element as any).webkitRequestFullscreen()
      } else if ((element as any).mozRequestFullScreen) {
        (element as any).mozRequestFullScreen()
      } else if ((element as any).msRequestFullscreen) {
        (element as any).msRequestFullscreen()
      }
      setIsFullscreen(true)
    }
  }

  const exitFullscreen = () => {
    if (document.exitFullscreen) {
      document.exitFullscreen()
    } else if ((document as any).webkitExitFullscreen) {
      (document as any).webkitExitFullscreen()
    } else if ((document as any).mozCancelFullScreen) {
      (document as any).mozCancelFullScreen()
    } else if ((document as any).msExitFullscreen) {
      (document as any).msExitFullscreen()
    }
    setIsFullscreen(false)
  }

  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement)
    }

    document.addEventListener("fullscreenchange", handleFullscreenChange)
    document.addEventListener("webkitfullscreenchange", handleFullscreenChange)
    document.addEventListener("mozfullscreenchange", handleFullscreenChange)
    document.addEventListener("MSFullscreenChange", handleFullscreenChange)

    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange)
      document.removeEventListener("webkitfullscreenchange", handleFullscreenChange)
      document.removeEventListener("mozfullscreenchange", handleFullscreenChange)
      document.removeEventListener("MSFullscreenChange", handleFullscreenChange)
    }
  }, [])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!showModal) return

      switch (e.key) {
        case "ArrowLeft":
          goToPrevious()
          break
        case "ArrowRight":
          goToNext()
          break
        case "Escape":
          closeModal()
          break
        case "f":
          toggleFullscreen()
          break
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => {
      window.removeEventListener("keydown", handleKeyDown)
    }
  }, [showModal, goToNext, goToPrevious, toggleFullscreen])

  const handleTouchStart = (e: React.TouchEvent) => {
    if (e.touches.length === 2) {
      setIsPinching(true)
      const dist = getDistanceBetweenTouches(e)
      setLastDistance(dist)
      return
    }

    setTouchEnd(null)
    setTouchStart(e.targetTouches[0].clientX)
    setTouchEndY(null)
    setTouchStartY(e.targetTouches[0].clientY)
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    if (isPinching && e.touches.length === 2) {
      e.preventDefault()
      const newDistance = getDistanceBetweenTouches(e)
      const scaleFactor = 0.01
      const newScale = scale + ((newDistance - lastDistance) * scaleFactor)
      
      if (newScale >= 0.5 && newScale <= 3) {
        setScale(newScale)
      }
      setLastDistance(newDistance)
      return
    }

    if (e.touches.length === 1) {
      setTouchEnd(e.targetTouches[0].clientX)
      setTouchEndY(e.targetTouches[0].clientY)
    }
  }

  const getDistanceBetweenTouches = (e: React.TouchEvent) => {
    if (e.touches.length < 2) return 0
    
    const dx = e.touches[0].clientX - e.touches[1].clientX
    const dy = e.touches[0].clientY - e.touches[1].clientY
    return Math.sqrt(dx * dx + dy * dy)
  }

  const handleTouchEnd = () => {
    setIsPinching(false)

    if (!touchStart || !touchEnd || !touchStartY || !touchEndY) return
    
    const horizontalDistance = touchStart - touchEnd
    const verticalDistance = touchStartY - touchEndY
    
    if (Math.abs(horizontalDistance) > Math.abs(verticalDistance)) {
      const isLeftSwipe = horizontalDistance > 50
      const isRightSwipe = horizontalDistance < -50
      
      if (isLeftSwipe) {
        goToNext()
      }
      
      if (isRightSwipe) {
        goToPrevious()
      }
    } else {
      const isUpSwipe = verticalDistance > 70
      
      if (isUpSwipe) {
        closeModal()
      }
    }
  }

  const resetZoom = () => {
    setScale(1)
  }

  const isSingleImage = bkeys.length === 1
  const gridColumns = bkeys.length === 2 ? 2 : bkeys.length === 3 ? 3 : 2
  const gridRows = bkeys.length <= 3 ? 1 : 2

  return (
    <div className={`c-image-gallery ${isSingleImage ? 'c-image-gallery--single' : ''}`}>
      {isSingleImage ? (
        <div className="c-image-gallery__single" onClick={() => openModal(0)}>
          <img src={uploadedImageURL(bkeys[0], 200)} alt="" loading="lazy" />
        </div>
      ) : (
        <div 
          className="c-image-gallery__grid"
          style={{
            gridTemplateColumns: `repeat(${gridColumns}, 1fr)`,
            gridTemplateRows: `repeat(${gridRows}, 1fr)`
          }}
        >
          {bkeys.slice(0, 4).map((bkey, index) => (
            <div 
              key={`${bkey}-${index}`} 
              className={`c-image-gallery__item ${bkeys.length === 3 && index === 2 ? 'c-image-gallery__item--wide' : ''}`}
              onClick={() => openModal(index)}
            >
              <img 
                src={uploadedImageURL(bkey, 200)} 
                alt="" 
                loading="lazy" 
              />
              {bkeys.length > 4 && index === 3 && (
                <div className="c-image-gallery__more">
                  +{bkeys.length - 4}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <Modal.Window 
        className="c-image-gallery-modal" 
        isOpen={showModal} 
        onClose={closeModal} 
        center={false} 
        size="fluid"
        manageHistory={false}
      >
        <Modal.Content>
          <div 
            ref={containerRef}
            className={`c-image-gallery-modal__container ${scale !== 1 ? 'c-image-gallery-modal__container--zoomed' : ''}`}
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
          >
            {!loadedPreview && <Loader />}
            <img 
              alt="" 
              onLoad={onPreviewLoad} 
              src={uploadedImageURL(bkeys[currentIndex], 1500)}
              style={{ transform: `scale(${scale})`, transformOrigin: 'center center', transition: isPinching ? 'none' : 'transform 0.2s ease' }}
            />
            
            <div className="c-image-gallery-modal__navigation">
              {bkeys.length > 1 && (
                <>
                  <Button 
                    variant="secondary" 
                    onClick={goToPrevious}
                    className="c-image-gallery-modal__nav-button c-image-gallery-modal__nav-button--prev"
                  >
                    <Trans id="action.previous">Previous</Trans>
                  </Button>
                  
                  <div className="c-image-gallery-modal__counter">
                    {currentIndex + 1} / {bkeys.length}
                  </div>
                  
                  <Button 
                    variant="secondary" 
                    onClick={goToNext}
                    className="c-image-gallery-modal__nav-button c-image-gallery-modal__nav-button--next"
                  >
                    <Trans id="action.next">Next</Trans>
                  </Button>
                </>
              )}

              <Button
                variant="secondary"
                onClick={toggleFullscreen}
                className="c-image-gallery-modal__nav-button c-image-gallery-modal__nav-button--fullscreen"
              >
                <Trans id={isFullscreen ? "action.exitfullscreen" : "action.fullscreen"}>
                  {isFullscreen ? "Exit Fullscreen" : "Fullscreen"}
                </Trans>
              </Button>

              {scale !== 1 && (
                <Button
                  variant="secondary"
                  onClick={resetZoom}
                  className="c-image-gallery-modal__nav-button c-image-gallery-modal__nav-button--reset-zoom"
                >
                  <Trans id="action.resetzoom">Reset Zoom</Trans>
                </Button>
              )}
            </div>
          </div>
        </Modal.Content>

        <Modal.Footer>
          <Button variant="tertiary" onClick={closeModal}>
            <Trans id="action.close">Close</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </div>
  )
}

