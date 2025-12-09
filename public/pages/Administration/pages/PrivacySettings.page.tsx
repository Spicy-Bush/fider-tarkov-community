import React, { useState } from "react"
import { Toggle, Form, Field } from "@fider/components"
import { actions, notify, Fider } from "@fider/services"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Privacy",
  subtitle: "Manage your site privacy",
  sidebarItem: "privacy",
}

const PrivacySettingsPage: React.FC = () => {
  const [isPrivate, setIsPrivate] = useState(Fider.session.tenant.isPrivate)

  const toggle = async (active: boolean) => {
    setIsPrivate(active)
    const response = await actions.updateTenantPrivacy(active)
    if (response.ok) {
      notify.success("Your privacy settings have been saved.")
    }
  }

  return (
    <Form>
      <Field label="Private Site">
        <Toggle active={isPrivate} onToggle={toggle} />
        <p className="text-muted mt-1">
          A private site prevents unauthenticated users from viewing or interacting with its content. <br /> When enabled, only already registered users,
          invited users and users from trusted OAuth providers will have access to this site.
        </p>
      </Field>
    </Form>
  )
}

export default PrivacySettingsPage
