
// pages will export their own pageConfig which will be extracted when the module loads
// see public\AsyncPages.tsx

export interface PageConfig {
  title?: string
  subtitle?: string
  sidebarItem?: string
  layoutVariant?: "default" | "fullWidth" | "custom"
}
