interface NavigationDestination {
  readonly url: string
  readonly key: string
  readonly sameDocument: boolean
  getState(): unknown
}

interface NavigationPrecommitController {
  redirect(url: string): void
}

interface NavigateEvent extends Event {
  readonly navigationType: "push" | "replace" | "reload" | "traverse"
  readonly destination: NavigationDestination
  readonly canIntercept: boolean
  readonly hashChange: boolean
  readonly downloadRequest: string | null
  readonly formData: FormData | null
  readonly sourceElement: Element | null
  readonly signal: AbortSignal
  intercept(options: {
    precommitHandler?: (controller: NavigationPrecommitController) => Promise<void>
    handler: () => Promise<void>
    focusReset?: "after-transition" | "manual"
    scroll?: "after-transition" | "manual"
  }): void
  scroll(): void
}

interface Navigation extends EventTarget {
  readonly currentEntry: NavigationHistoryEntry
  updateCurrentEntry(options: { state: unknown }): void
  traverseTo(key: string): { committed: Promise<NavigationHistoryEntry>; finished: Promise<NavigationHistoryEntry> }
}

interface Window {
  navigation?: Navigation
  NavigationPrecommitController?: { prototype: NavigationPrecommitController }
}
