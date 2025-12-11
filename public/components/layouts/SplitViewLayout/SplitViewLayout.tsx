import React from "react"
import { classSet } from "@fider/services"
import { Button, Icon, Loader } from "@fider/components"
import { heroiconsChevronUp as IconChevron } from "@fider/icons.generated"

import "./SplitViewLayout.scss"

export interface SplitViewLayoutProps {
  listWidth?: number
  children: React.ReactNode
}

export const SplitViewLayout: React.FC<SplitViewLayoutProps> = ({
  listWidth = 400,
  children,
}) => {
  return (
    <div
      className="c-split-view"
      style={{ "--split-view-list-width": `${listWidth}px` } as React.CSSProperties}
    >
      {children}
    </div>
  )
}

export interface SplitViewListPaneProps {
  header?: React.ReactNode
  footer?: React.ReactNode
  isLoading?: boolean
  emptyState?: React.ReactNode
  children: React.ReactNode
}

export const SplitViewListPane: React.FC<SplitViewListPaneProps> = ({
  header,
  footer,
  isLoading,
  emptyState,
  children,
}) => {
  const hasContent = React.Children.count(children) > 0

  return (
    <div className="c-split-view__list">
      {header && <div className="c-split-view__list-header">{header}</div>}
      <div className="c-split-view__list-content">
        {isLoading ? (
          <div className="c-split-view__list-loading">
            <Loader />
          </div>
        ) : hasContent ? (
          children
        ) : (
          emptyState && (
            <div className="c-split-view__list-empty">{emptyState}</div>
          )
        )}
      </div>
      {footer && <div className="c-split-view__list-footer">{footer}</div>}
    </div>
  )
}

export interface OverlayConfig {
  key: string
  content: React.ReactNode
  isOpen: boolean
}

export interface SplitViewPreviewPaneProps {
  hasSelection: boolean
  onMobileBack?: () => void
  emptyState?: React.ReactNode
  isLoading?: boolean
  overlays?: OverlayConfig[]
  children: React.ReactNode
}

export const SplitViewPreviewPane: React.FC<SplitViewPreviewPaneProps> = ({
  hasSelection,
  onMobileBack,
  emptyState,
  isLoading,
  overlays = [],
  children,
}) => {
  const className = classSet({
    "c-split-view__preview": true,
    "c-split-view__preview--mobile-open": hasSelection,
  })

  return (
    <div className={className}>
      {hasSelection && onMobileBack && (
        <Button
          variant="tertiary"
          size="small"
          className="c-split-view__mobile-back"
          onClick={onMobileBack}
        >
          <Icon sprite={IconChevron} className="c-split-view__mobile-back-icon" />
          <span>Back to list</span>
        </Button>
      )}
      {isLoading ? (
        <div className="c-split-view__preview-loading">
          <Loader />
        </div>
      ) : hasSelection ? (
        children
      ) : (
        emptyState && (
          <div className="c-split-view__preview-empty">{emptyState}</div>
        )
      )}
      {overlays.map((overlay) => (
        <div
          key={overlay.key}
          className={classSet({
            "c-split-view__overlay": true,
            "c-split-view__overlay--open": overlay.isOpen,
          })}
        >
          {overlay.content}
        </div>
      ))}
    </div>
  )
}

