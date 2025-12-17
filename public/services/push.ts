import { http, Result } from "@fider/services"

interface VAPIDKeyResponse {
  enabled: boolean
  publicKey?: string
}

interface PushStatusResponse {
  enabled: boolean
  subscribed: boolean
}

let vapidKeyCache: VAPIDKeyResponse | null = null

export const getVAPIDPublicKey = async (): Promise<VAPIDKeyResponse> => {
  if (vapidKeyCache) {
    return vapidKeyCache
  }

  const result = await http.get<VAPIDKeyResponse>("/_api/push/vapid-key")
  if (result.ok) {
    vapidKeyCache = result.data
    return result.data
  }
  return { enabled: false }
}

export const getPushStatus = async (): Promise<Result<PushStatusResponse>> => {
  return http.get<PushStatusResponse>("/_api/push/status")
}

export const isPushSupported = (): boolean => {
  return "serviceWorker" in navigator && "PushManager" in window && "Notification" in window
}

export const getNotificationPermission = (): NotificationPermission => {
  if (!("Notification" in window)) {
    return "denied"
  }
  return Notification.permission
}

export const requestNotificationPermission = async (): Promise<NotificationPermission> => {
  if (!("Notification" in window)) {
    return "denied"
  }
  return Notification.requestPermission()
}

export const subscribeToPush = async (): Promise<Result> => {
  if (!isPushSupported()) {
    return { ok: false, data: undefined, error: { errors: [{ message: "Push notifications are not supported" }] } }
  }

  const vapidKey = await getVAPIDPublicKey()
  if (!vapidKey.enabled || !vapidKey.publicKey) {
    return { ok: false, data: undefined, error: { errors: [{ message: "Push notifications are not enabled on this server" }] } }
  }

  const permission = await requestNotificationPermission()
  if (permission !== "granted") {
    return { ok: false, data: undefined, error: { errors: [{ message: "Notification permission denied" }] } }
  }

  try {
    const registration = await navigator.serviceWorker.ready
    const existingSubscription = await registration.pushManager.getSubscription()
    
    if (existingSubscription) {
      await existingSubscription.unsubscribe()
    }

    const subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(vapidKey.publicKey) as BufferSource,
    })

    const subJson = subscription.toJSON()
    return http.post("/_api/push/subscribe", {
      endpoint: subJson.endpoint,
      keys: {
        p256dh: subJson.keys?.p256dh,
        auth: subJson.keys?.auth,
      },
    })
  } catch (err) {
    return { ok: false, data: undefined, error: { errors: [{ message: "Failed to subscribe to push notifications" }] } }
  }
}

export const unsubscribeFromPush = async (): Promise<Result> => {
  if (!isPushSupported()) {
    return { ok: true, data: undefined }
  }

  try {
    const registration = await navigator.serviceWorker.ready
    const subscription = await registration.pushManager.getSubscription()
    
    if (subscription) {
      const endpoint = subscription.endpoint
      await subscription.unsubscribe()
      return http.delete("/_api/push/subscribe", { endpoint })
    }
    
    return { ok: true, data: undefined }
  } catch (err) {
    return { ok: false, data: undefined, error: { errors: [{ message: "Failed to unsubscribe from push notifications" }] } }
  }
}

export const getCurrentSubscription = async (): Promise<PushSubscription | null> => {
  if (!isPushSupported()) {
    return null
  }

  try {
    const registration = await navigator.serviceWorker.ready
    return registration.pushManager.getSubscription()
  } catch {
    return null
  }
}

export const registerServiceWorker = async (): Promise<ServiceWorkerRegistration | null> => {
  if (!("serviceWorker" in navigator)) {
    return null
  }

  try {
    const registration = await navigator.serviceWorker.register("/sw.js", { scope: "/" })
    return registration
  } catch {
    return null
  }
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/")
  const rawData = window.atob(base64)
  const outputArray = new Uint8Array(rawData.length)
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i)
  }
  return outputArray
}

