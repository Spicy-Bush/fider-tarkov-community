import React from "react"
import { Button, Icon } from "@fider/components"
import { heroiconsDownload as IconDownload } from "@fider/icons.generated"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Export",
  subtitle: "Download your data",
  sidebarItem: "export",
}

const ExportPage: React.FC = () => {
  return (
    <>
      <h2 className="text-display">Export Posts</h2>
      <p className="text-muted">
        Use this button to download a CSV file with all posts in this site. This can be useful to analyse the data with an external tool or simply to back it
        up.
      </p>
      <div className="c-admin-actions">
        <Button variant="secondary" href="/admin/export/posts.csv">
          <Icon sprite={IconDownload} />
          <span>posts.csv</span>
        </Button>
      </div>

      <div className="mt-8">
        <h2 className="text-display">Backup your data</h2>
        <p className="text-muted">
          Use this button to download a ZIP file with your data in JSON format. This is a full backup and contains all of your data.
        </p>
        <div className="c-admin-actions">
          <Button variant="secondary" href="/admin/export/backup.zip">
            <Icon sprite={IconDownload} />
            <span>backup.zip</span>
          </Button>
        </div>
      </div>
    </>
  )
}

export default ExportPage
