import { createContext } from "react"
import { CurrentUser, SessionPermissions, SystemSettings, Tenant, UserProfileStandingResponse } from "@fider/models"
import { TenantStatus } from "@fider/models/identity"

export interface ServerData {
  title: string
  description?: string
  canonicalURL?: string
  page: string
  contextID: string
  props: Record<string, any>
  tenant: Tenant
  user?: CurrentUser
  permissions: SessionPermissions
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

  public get permissions(): SessionPermissions {
    return this.data.permissions
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

  public updateUserProfile(change: Partial<Pick<CurrentUser, "name" | "avatarURL" | "avatarType" | "visualRole" | "visualRoleOverride">>): void {
    const user = this.user
    const changed = Object.entries(change).some(([key, value]) => user[key as keyof CurrentUser] !== value)

    if (!changed) {
      return
    }

    this.refresh({ ...this.data, user: { ...user, ...change } })
  }

  public updateUserStanding(standing: UserProfileStandingResponse): void {
    const warning = standing.warnings.find((entry) => entry.isActive)
    const mute = standing.mutes.find((entry) => entry.isActive)

    this.refresh({
      ...this.data,
      permissions: standing.sessionPermissions,
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
    return this
  }

  public refresh(data: ServerData): void {
    this.pSession.refresh(data)
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
