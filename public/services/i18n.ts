import { i18n } from "@lingui/core"

export function activateI18NSync(locale: string, messages?: any) {
  i18n.load(locale, messages)
  i18n.activate(locale)
  return i18n
}

export async function activateI18N(locale: string) {
  try {
    const { messages } = await import(`@locale/${locale}/client.mjs`)
    return activateI18NSync(locale, messages)
  } catch (err) {
    console.error(err)
    return activateI18NSync(locale)
  }
}
