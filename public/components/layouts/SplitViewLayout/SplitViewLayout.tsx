// SplitViewLayout converted to Tailwind

import React from "react"
import { classSet } from "@fider/services"
import { Button, Icon, Loader } from "@fider/components"
import { heroiconsChevronUp as IconChevron } from "@fider/icons.generated"

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
      className="flex flex-col lg:flex-row gap-4 h-[calc(100vh-130px)] min-h-[500px]"
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
    <div className="flex-[0_0_100%] lg:flex-[0_0_var(--split-view-list-width)] lg:min-w-[var(--split-view-list-width)] lg:h-full flex flex-col bg-elevated rounded-panel border border-surface-alt overflow-hidden">
      {header && (
        <div className="flex items-center justify-between p-3 border-b border-surface-alt bg-tertiary shrink-0">
          {header}
        </div>
      )}
      <div className="flex-1 overflow-y-auto min-h-0">
        {isLoading ? (
          <div className="flex items-center justify-center p-8 text-border-strong">
            <Loader />
          </div>
        ) : hasContent ? (
          children
        ) : (
          emptyState && (
            <div className="flex items-center justify-center p-8 text-border-strong">{emptyState}</div>
          )
        )}
      </div>
      {footer && (
        <div className="flex items-center justify-between py-2 px-3 border-t border-surface-alt bg-tertiary shrink-0">
          {footer}
        </div>
      )}
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
  return (
    <div className={classSet({
      "flex-1 min-w-0 h-full overflow-y-auto bg-elevated rounded-panel border border-surface-alt relative lg:max-h-[90vh]": true,
      "max-lg:fixed max-lg:inset-0 max-lg:z-modal max-lg:bg-tertiary max-lg:p-4": hasSelection,
    })}>
      {hasSelection && onMobileBack && (
        <Button
          variant="tertiary"
          size="small"
          className="hidden max-lg:flex mb-3"
          onClick={onMobileBack}
        >
          <Icon sprite={IconChevron} className="-rotate-90 w-4 h-4" />
          <span>Back to list</span>
        </Button>
      )}
      {isLoading ? (
        <div className="flex items-center justify-center min-h-[400px] text-border-strong">
          <Loader />
        </div>
      ) : hasSelection ? (
        children
      ) : (
        emptyState && (
          <div className="flex items-center justify-center min-h-[400px] text-border-strong">{emptyState}</div>
        )
      )}
      {overlays.map((overlay) => (
        <div
          key={overlay.key}
          className={classSet({
            "absolute inset-0 z-50 bg-elevated overflow-y-auto rounded-panel transition-all duration-75": true,
            "opacity-100 visible translate-x-0": overlay.isOpen,
            "opacity-0 invisible translate-x-5": !overlay.isOpen,
          })}
        >
          {overlay.content}
        </div>
      ))}
    </div>
  )
}
