import React, { useState } from "react"
import { TextArea, Form, Button } from "@fider/components"
import { Failure, actions, Fider } from "@fider/services"
import { CollapsiblePanel } from "@fider/components/common/CollapsiblePanel"
import { PageConfig } from "@fider/components/layouts"

interface AdvancedSettingsPageProps {
  customCSS: string
  profanityWords: string
}

export const pageConfig: PageConfig = {
  title: "Advanced",
  subtitle: "Manage your site settings",
  sidebarItem: "advanced",
}

const AdvancedSettingsPage: React.FC<AdvancedSettingsPageProps> = (props) => {
  const [customCSS, setCustomCSS] = useState(props.customCSS)
  const [profanityWords, setProfanityWords] = useState(props.profanityWords)
  const [error, setError] = useState<Failure | undefined>()

  const handleSaveProfanityWords = async (): Promise<void> => {
    const result = await actions.updateProfanityWords(profanityWords)
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }

  const handleSaveCustomCSS = async (): Promise<void> => {
    const result = await actions.updateTenantAdvancedSettings(customCSS)
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }

  return (
    <Form error={error}>
      <CollapsiblePanel title="Custom CSS" defaultOpen={false}>
        <TextArea
          field="customCSS"
          label="Custom CSS"
          disabled={!Fider.session.user.isAdministrator}
          minRows={10}
          value={customCSS}
          onChange={setCustomCSS}
        >
          {}
        </TextArea>

        <div className="c-admin-actions">
          <Button variant="primary" onClick={handleSaveCustomCSS}>
            Save Custom CSS
          </Button>
        </div>
      </CollapsiblePanel>

      <TextArea
        field="profanityWords"
        label="Profanity Words (one per line)"
        disabled={!Fider.session.user.isAdministrator}
        minRows={5}
        value={profanityWords}
        onChange={setProfanityWords}
      >
        <p className="text-muted">
          Enter banned words, one per line. Any post or comment containing these words will be blocked.
        </p>
      </TextArea>

      <div className="c-admin-actions">
        <Button variant="primary" onClick={handleSaveProfanityWords}>
          Save Profanity Words
        </Button>
      </div>
    </Form>
  )
}

export default AdvancedSettingsPage
