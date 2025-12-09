import React, { useState } from "react"

import { Button, ButtonClickEvent, TextArea, Form, Input, ImageUploader, Select } from "@fider/components"
import { actions, Failure, Fider } from "@fider/services"
import { ImageUpload } from "@fider/models"
import { useFider } from "@fider/hooks"
import locales from "@locale/locales"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "General",
  subtitle: "Manage your site settings",
  sidebarItem: "general",
}

const GeneralSettingsPage = () => {
  const fider = useFider()
  const [title, setTitle] = useState<string>(fider.session.tenant.name)
  const [welcomeMessage, setWelcomeMessage] = useState<string>(fider.session.tenant.welcomeMessage)
  const [messageBanner, setMessageBanner] = useState<string>(fider.session.tenant.messageBanner)
  const [invitation, setInvitation] = useState<string>(fider.session.tenant.invitation)
  const [logo, setLogo] = useState<ImageUpload | undefined>(undefined)
  const [cname, setCNAME] = useState<string>(fider.session.tenant.cname)
  const [locale, setLocale] = useState<string>(fider.session.tenant.locale)
  const [error, setError] = useState<Failure | undefined>(undefined)

  const isAdmin = fider.session.user.isAdministrator

  const handleSave = async (e: ButtonClickEvent) => {
    let result: any = { ok: true };
    
    if (isAdmin) {
      result = await actions.updateTenantSettings({ 
        title, 
        cname, 
        welcomeMessage, 
        invitation, 
        logo, 
        locale 
      });
    }
    
    const messageBannerResult = await actions.updateTenantMessageBanner(messageBanner);
    
    if (messageBannerResult.error) {
      setError(messageBannerResult.error);
    }
    
    if (result.ok) {
      e.preventEnable();
      location.href = `/`;
    } else if (result.error) {
      setError(result.error);
    }
  }

  const dnsInstructions = (): JSX.Element => {
    const isApex = cname.split(".").length <= 2
    const recordType = isApex ? "ALIAS" : "CNAME"
    return (
      <>
        <strong>{cname}</strong> {recordType}{" "}
        <strong>
          {fider.session.tenant.subdomain}
          {fider.settings.domain}
        </strong>
      </>
    )
  }

  return (
    <Form error={error}>
      {isAdmin && (
        <>
          <Input field="title" label="Your Fider board's title" maxLength={60} value={title} onChange={setTitle}>
            <p className="text-muted">Keep it short and snappy. Your product / service name is usually best.</p>
          </Input>
        </>
      )}

      <TextArea
        field="messageBanner"
        label="Message Banner"
        value={messageBanner}
        maxLength={1000}
        onChange={setMessageBanner}
      ></TextArea>

      {isAdmin && (
        <>
          <TextArea
            field="welcomeMessage"
            label="Welcome Message"
            value={welcomeMessage}
            onChange={setWelcomeMessage}
          >
            <p className="text-muted">
              The message is shown on this site&apos;s home page. Use it to help visitors understand what this space is about and the importance of their
              feedback.
            </p>
          </TextArea>

          <Input
            field="invitation"
            label="Invitation"
            maxLength={60}
            value={invitation}
            placeholder="Enter your suggestion here..."
            onChange={setInvitation}
          >
            <p className="text-muted">Placeholder text in the suggestion&apos;s box. It should invite your visitors into sharing their feedback.</p>
          </Input>

          <ImageUploader label="Your Logo" field="logo" bkey={fider.session.tenant.logoBlobKey} onChange={setLogo}>
            <p className="text-muted">JPG or PNG smaller than 5MB, minimum size 200x200 pixels.</p>
          </ImageUploader>

          {!Fider.isSingleHostMode() && (
            <Input
              field="cname"
              label="Custom Domain"
              maxLength={100}
              placeholder="feedback.yourcompany.com"
              value={cname}
              onChange={setCNAME}
            >
              <div className="text-muted">
                {cname ? (
                  [
                    <p key={0}>Enter the following record into your DNS zone records:</p>,
                    <p key={1}>{dnsInstructions()}</p>,
                    <p key={2}>Please note that it may take up to 72 hours for the change to take effect worldwide due to DNS propagation.</p>,
                  ]
                ) : (
                  <p>
                    Use custom domains to access Fider via your own domain name <code>feedback.yourcompany.com</code>
                  </p>
                )}
              </div>
            </Input>
          )}

          <Select
            label="Locale"
            field="locale"
            defaultValue={locale}
            options={Object.entries(locales).map(([k, v]) => ({
              value: k,
              label: v.text,
            }))}
            onChange={(o) => setLocale(o?.value || "en")}
          >
            {locale !== "en" && (
              <>
                <p className="text-muted">
                  This language is translated by the Open Source community. If you find a mistake or would like to improve its quality, visit{" "}
                  <a className="text-link" target="_blank" rel="noopener" href="https://crowdin.com/project/fider">
                    Crowdin
                  </a>{" "}
                  and contribute with your own translations. No technical knowledge is required.
                </p>
                <p className="text-muted">Only public pages are translated. Internal and/or administrative pages will remain in English.</p>
              </>
            )}
          </Select>
        </>
      )}

      <div className="field">
        <Button variant="primary" onClick={handleSave}>
          Save
        </Button>
      </div>
    </Form>
  )
}

export default GeneralSettingsPage
