import React from "react"

// defines DOM related expect methods
import "@testing-library/jest-dom"

// Mock modules that use import.meta.glob (Vite-specific feature)
jest.mock("@fider/contexts/AdminNavigationContext", () => ({
  useAdminNavigation: () => ({
    currentPage: null,
    navigate: async () => {},
    prefetch: () => {},
  }),
}))

jest.mock("@fider/AsyncPages", () => ({
  usePageLoader: () => ({
    Component: null,
    pageConfig: undefined,
    isLoading: false,
    error: null,
  }),
  AsyncPageLoader: () => null,
}))

// Mock for LinguiJS so we don't need to setup i18n on each test
jest.mock("@lingui/react", () => ({
  Trans: function TransMock({ children }: { children: React.ReactNode }) {
    return <>{children}</>
  },

  t: function tMock(id: string): string {
    return id
  },

  Plural: function PluralMock({ value, one, other }: { value: number; one: React.ReactNode; other: React.ReactNode }) {
    return <>{value > 1 ? other : one}</>
  },
}))

// Mock for marked (ESM module that Jest can't handle)
jest.mock("marked", () => {
  class MarkedMock {
    parse(markdown: string): string {
      return `<p>${markdown}</p>`
    }
    use(options: any): MarkedMock {
      return this
    }
  }
  return {
    Marked: MarkedMock,
  }
})
