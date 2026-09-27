import { createContext } from "react"
import { CurrentUser, SystemSettings, Tenant, TenantStatus, UserProfileStanding } from "@fider/models"

export interface ServerData {
  title: string
  description?: string
  canonicalURL?: string
  page: string
  contextID: string
  props: Record<string, any>
  tenant: Tenant
  user?: CurrentUser
  settings: SystemSettings
}

export class FiderSession {
  private data: ServerData
  private listeners = new Set<() => void>()

  constructor(data: ServerData) {
    this.data = data
  }

  public get page(): string {
    return this.data.page
  }

  public get contextID(): string {
    return this.data.contextID
  }

  public get user(): CurrentUser {
    if (!this.data.user) throw new Error("User is undefined")
    return this.data.user
  }

  public getSnapshot = (): ServerData => this.data

  public getUserSnapshot = (): CurrentUser | undefined => this.data.user

  public refresh(data: ServerData): void {
    this.data = data

    for (const listener of this.listeners) {
      listener()
    }
  }

  public subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)

    return () => {
      this.listeners.delete(listener)
    }
  }

  public updateUserProfile(change: Partial<Pick<CurrentUser, "name" | "avatarURL" | "avatarType" | "visualRole">>): void {
    const user = this.user
    const changed = Object.entries(change).some(([key, value]) => user[key as keyof CurrentUser] !== value)

    if (!changed) {
      return
    }

    this.refresh({ ...this.data, user: { ...user, ...change } })
  }

  public updateUserStanding(standing: UserProfileStanding): void {
    const warning = standing.warnings.find((entry) => entry.isActive)
    const mute = standing.mutes.find((entry) => entry.isActive)

    this.refresh({
      ...this.data,
      user: {
        ...this.user,
        hasWarning: !!warning,
        latestWarningId: warning?.id,
        isMuted: !!mute,
        latestMuteId: mute?.id,
      },
    })
  }

  public get tenant(): Tenant {
    return this.data.tenant
  }

  public get props(): { [key: string]: any } {
    return this.data.props
  }

  public get isAuthenticated(): boolean {
    return !!this.data.user
  }
}

export class FiderImpl {
  private pSession!: FiderSession

  public initialize = (initData?: any): FiderImpl => {
    let data = initData

    if (!data) {
      const element = document.getElementById("server-data")
      data = element ? JSON.parse(element.textContent || element.innerText) : {}
    }

    this.pSession = new FiderSession(data)
    this.syncAdSenseClient()
    return this
  }

  public refresh(data: ServerData): void {
    this.pSession.refresh(data)
    this.syncAdSenseClient()
  }

  private syncAdSenseClient(): void {
    if (typeof window === "undefined") return

    window.__adsense_client = (this.settings?.googleAdSense || "").trim()
  }

  public get currentLocale(): string {
    if (this.session.tenant) {
      return this.session.tenant.locale
    }
    return this.settings.locale
  }

  public get session(): FiderSession {
    return this.pSession
  }

  public get settings(): SystemSettings {
    return this.pSession.getSnapshot().settings
  }

  public get isReadOnly(): boolean {
    return this.session.tenant && this.session.tenant.status === TenantStatus.Locked
  }

  public isProduction(): boolean {
    return this.settings.environment === "production"
  }

  public isSingleHostMode(): boolean {
    return this.settings.mode === "single"
  }
}

export const Fider = new FiderImpl()

export const FiderContext = createContext<FiderImpl>(Fider)
