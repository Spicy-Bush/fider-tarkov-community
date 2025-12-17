import React, { useState, useEffect } from "react"
import { useFider } from "@fider/hooks"
import { HStack } from "@fider/components/layout"
import { NavigationLink } from "@fider/models"

const STORAGE_KEY = "subheader_open"

export const SubheaderBar = () => {
  const fider = useFider()
  const links = (fider.settings.navigationLinks || []).filter((l: NavigationLink) => l.location === "subheader")
  const [isOpen, setIsOpen] = useState(() => {
    if (typeof window === "undefined") return true
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored === null ? true : stored === "true"
  })

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(isOpen))
  }, [isOpen])
  
  if (links.length === 0) return null

  const linkClass = "text-sm text-muted hover:text-foreground px-2 md:px-3 py-2 rounded-button hover:bg-tertiary-hover whitespace-nowrap shrink-0"

  return (
    <div className="relative">
      <div className="lg:hidden bg-tertiary border-b border-border">
        <div className="px-2">
          <div className="min-h-[48px] py-2 flex items-center overflow-x-auto scrollbar-none">
            <div className="flex gap-1 mx-auto">
              {links.map((link) => (
                <a key={link.id} href={link.url} className={linkClass}>
                  {link.title}
                </a>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="hidden lg:block">
        <div
          className={`bg-tertiary border-b border-border overflow-hidden transition-all duration-300 ${
            isOpen ? "max-h-12" : "max-h-0 border-b-0"
          }`}
        >
          <div className="container">
            <div className="h-12 flex items-center justify-center">
              <HStack spacing={4}>
                {links.map((link) => (
                  <a key={link.id} href={link.url} className={linkClass}>
                    {link.title}
                  </a>
                ))}
              </HStack>
            </div>
          </div>
        </div>

        <div className="container relative">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="cursor-pointer absolute -right-12 top-0 bg-tertiary border border-t-0 border-border rounded-b px-3 py-1 hover:bg-tertiary-hover transition-colors z-40"
            aria-label={isOpen ? "Hide navigation" : "Show navigation"}
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 16 16"
              fill="currentColor"
              className={`text-muted transition-transform duration-300 cursor-pointer ${isOpen ? "" : "rotate-180"}`}
            >
              <path d="M4 10l4-4 4 4H4z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
