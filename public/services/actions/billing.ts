import { http, Result } from "../http"

interface CheckoutPageLink {
  url: string
}

export const generateCheckoutLink = async (planId: string): Promise<Result<CheckoutPageLink>> => {
  return await http.post("/api/billing/checkout-link", { planId })
}
