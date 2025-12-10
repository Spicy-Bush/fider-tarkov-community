interface GetPriceResponse {
  price: { net: string }
  error?: {
    message: string
  }
}

interface PaddleSdk {
  isReady: boolean
  Setup(params: { vendor: number }): void
  Environment: {
    set(envName: "sandbox"): void
  }
  Checkout: {
    open(params: { override: string; closeCallback: () => void }): void
  }
  Product: {
    Prices(planId: number, callback: (resp: GetPriceResponse) => void): void
  }
}
declare interface Window {
  ga?: (cmd: string, evt: string, args?: any) => void
  set: (key: string, value: any) => void
  Paddle: PaddleSdk
}

interface SpriteSymbol {
  id: string
  viewBox: string
}

