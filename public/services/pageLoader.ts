import { ComponentType } from "react"
import { PageConfig } from "@fider/components/layouts"
import { RETRY } from "./constants"

export interface PageModule {
  default: ComponentType<any>
  pageConfig?: PageConfig
}

export interface PageLoaderConfig {
  pathPrefix: string
}

type ModuleGlob = Record<string, () => Promise<PageModule>>

const retry = <T,>(
  fn: () => Promise<T>,
  retriesLeft = RETRY.MAX_PAGE_LOAD_RETRIES,
  waitMs = RETRY.PAGE_LOAD_RETRY_INTERVAL_MS
): Promise<T> => {
  return new Promise((resolve, reject) => {
    fn()
      .then(resolve)
      .catch((err) => {
        setTimeout(() => {
          if (retriesLeft === 1) {
            reject(new Error(`${err} after ${RETRY.MAX_PAGE_LOAD_RETRIES} retries`))
            return
          }
          retry(fn, retriesLeft - 1, waitMs + RETRY.PAGE_LOAD_RETRY_INTERVAL_MS).then(resolve, reject)
        }, waitMs)
      })
  })
}

export interface PageLoader {
  load: (pageName: string) => Promise<PageModule>
  prefetch: (pageName: string) => void
  getCached: (pageName: string) => PageModule | undefined
}

export const createPageLoader = (modules: ModuleGlob, config: PageLoaderConfig): PageLoader => {
  const moduleCache = new Map<string, PageModule>()
  const loadingPromises = new Map<string, Promise<PageModule>>()

  const load = (pageName: string): Promise<PageModule> => {
    const cached = moduleCache.get(pageName)
    if (cached) {
      return Promise.resolve(cached)
    }

    const existingPromise = loadingPromises.get(pageName)
    if (existingPromise) {
      return existingPromise
    }

    const modulePath = `${config.pathPrefix}${pageName}.tsx`
    const loader = modules[modulePath]
    if (!loader) {
      return Promise.reject(new Error(`Page not found: ${pageName}`))
    }

    const loadPromise = retry(() => loader()).then((module: PageModule) => {
      moduleCache.set(pageName, module)
      loadingPromises.delete(pageName)
      return module
    })

    loadingPromises.set(pageName, loadPromise)
    return loadPromise
  }

  const prefetch = (pageName: string): void => {
    if (moduleCache.has(pageName) || loadingPromises.has(pageName)) return
    load(pageName).catch(() => {})
  }

  const getCached = (pageName: string): PageModule | undefined => {
    return moduleCache.get(pageName)
  }

  return { load, prefetch, getCached }
}

