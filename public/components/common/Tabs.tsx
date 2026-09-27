import React, { useEffect, useLayoutEffect, useRef, useState } from "react"
import { classSet } from "@fider/services"

interface TabItem<K extends string> {
  value: K
  label: React.ReactNode
  counter?: number
}

interface TabsProps<K extends string> {
  tabs: readonly TabItem<K>[]
  activeTab: K
  onChange: (value: K) => void
  className?: string
  listClassName?: string
  tabClassName?: (selected: boolean) => string
  indicator?: "underline" | "pill"
  indicatorClassName?: string
}

export function Tabs<K extends string>({
  tabs,
  activeTab,
  onChange,
  className,
  listClassName = "flex border-b border-surface-alt",
  tabClassName,
  indicator = "underline",
  indicatorClassName = "bg-primary",
}: TabsProps<K>) {
  const listRef = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState<IndicatorBox | null>(null)

  useLayoutEffect(() => {
    const list = listRef.current!

    const measure = () => {
      const tab = list.querySelector<HTMLButtonElement>(":scope > button[aria-selected=true]")
      let next: IndicatorBox | null = null

      if (tab) {
        next = {
          x: tab.offsetLeft,
          y: tab.offsetTop,
          width: tab.offsetWidth,
          height: tab.offsetHeight,
        }
      }

      setBox((previous) => {
        if (
          previous && next &&
          previous.x === next.x &&
          previous.y === next.y &&
          previous.width === next.width &&
          previous.height === next.height
        ) {
          return previous
        }

        return next
      })
    }

    measure()

    const observer = new ResizeObserver(measure)
    observer.observe(list)
    list.querySelectorAll("[role=tab]").forEach((tab) => observer.observe(tab))

    return () => observer.disconnect()
  }, [activeTab, tabs])

  return (
    <div className={className}>
      <div ref={listRef} role="tablist" className={`relative ${listClassName}`}>
        <TabIndicator box={box} variant={indicator} className={indicatorClassName} />
        {tabs.map((tab) => {
          const isActive = activeTab === tab.value
          const buttonClassName = tabClassName
            ? tabClassName(isActive)
            : classSet({
                "py-2.5 px-4 mr-2 cursor-pointer flex items-center font-medium transition-colors duration-150 ease-out border-b-2 border-transparent": true,
                "text-subtle hover:text-foreground": !isActive,
                "text-primary": isActive,
              })

          return (
            <button
              type="button"
              role="tab"
              aria-selected={isActive}
              key={tab.value}
              className={`relative ${buttonClassName}`}
              onClick={() => {
                if (!isActive) {
                  onChange(tab.value)
                }
              }}
            >
              {tab.label}
              {tab.counter !== undefined && (
                <span
                  className={classSet({
                    "ml-1.5 py-0.5 px-1.5 rounded-badge text-[11px] min-w-5 text-center transition-colors duration-150 ease-out": true,
                    "bg-surface-alt text-muted": !isActive,
                    "bg-accent-light text-primary-hover": isActive,
                  })}
                >
                  {tab.counter}
                </span>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}

interface IndicatorBox {
  x: number
  y: number
  width: number
  height: number
}

interface TabIndicatorProps {
  box: IndicatorBox | null
  variant: "underline" | "pill"
  className?: string
}

const TabIndicator = ({ box, variant, className }: TabIndicatorProps) => {
  // Avoid animating from the unmeasured position.
  const [placed, setPlaced] = useState(false)

  useEffect(() => {
    if (!box || placed) return
    const frame = requestAnimationFrame(() => setPlaced(true))
    return () => cancelAnimationFrame(frame)
  }, [box, placed])

  if (!box) return null

  const style: React.CSSProperties =
    variant === "underline"
      ? // Keep hovered tab backgrounds behind the underline.
        {
          transform: `translate(${box.x}px, ${box.y + box.height - 2}px)`,
          width: box.width,
          height: 2,
          zIndex: 1,
        }
      : {
          transform: `translate(${box.x}px, ${box.y}px)`,
          width: box.width,
          height: box.height,
        }

  return (
    <span
      aria-hidden="true"
      className={`absolute left-0 top-0 pointer-events-none ${placed ? "tab-indicator-move" : ""} ${className || ""}`}
      style={style}
    />
  )
}

interface TabPanelsProps<K extends string> {
  /** Slide direction follows the visual order of these keys. */
  keys: readonly K[]
  activeKey: K
  keepMounted?: boolean
  className?: string
  panelClassName?: string
  children: (key: K) => React.ReactNode
}

type Slide<K> = { from: K | null; to: K; direction: "next" | "prev" }

export function TabPanels<K extends string>({ keys, activeKey, keepMounted = false, className, panelClassName, children }: TabPanelsProps<K>) {
  const [slide, setSlide] = useState<Slide<K>>({ from: null, to: activeKey, direction: "next" })

  if (slide.to !== activeKey) {
    setSlide({ from: slide.to, to: activeKey, direction: keys.indexOf(activeKey) > keys.indexOf(slide.to) ? "next" : "prev" })
  }

  const leaving = slide.from !== null && slide.from !== activeKey ? slide.from : null

  // Hidden panels do not emit animationend.
  useEffect(() => {
    if (leaving === null) return
    const timer = window.setTimeout(() => setSlide((s) => ({ ...s, from: null })), 400)
    return () => window.clearTimeout(timer)
  }, [leaving, slide])

  const finish = (event: React.AnimationEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) {
      setSlide((s) => ({ ...s, from: null }))
    }
  }

  return (
    // Idle panels must let popovers extend outside their bounds.
    <div className={`relative ${leaving !== null ? "overflow-clip" : ""} ${className || ""}`}>
      {keys.map((key) => {
        const isActive = key === activeKey
        const isLeaving = key === leaving
        if (!isActive && !isLeaving && !keepMounted) return null

        // Prevent margin collapse from shifting a panel when it becomes absolute.
        const motion = isActive
          ? `flow-root ${leaving !== null ? `tab-slide-in-${slide.direction}` : ""}`
          : isLeaving
            ? `flow-root tab-slide-out-${slide.direction} absolute inset-x-0 top-0`
            : "hidden"

        return (
          <div
            key={key}
            role="tabpanel"
            className={`${motion} ${panelClassName || ""}`}
            aria-hidden={isActive ? undefined : true}
            {...(isLeaving ? { inert: "" } : {})}
            onAnimationEnd={isLeaving ? finish : undefined}
          >
            {children(key)}
          </div>
        )
      })}
    </div>
  )
}
