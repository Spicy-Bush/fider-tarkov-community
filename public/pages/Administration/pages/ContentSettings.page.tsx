import React, { useState } from "react"
import { Button, ButtonClickEvent, Form, Input, Toggle } from "@fider/components"
import { actions, Failure, classSet } from "@fider/services"
import { useFider } from "@fider/hooks"
import { CollapsiblePanel } from "@fider/components/common/CollapsiblePanel"
import { HStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"

import "./ContentSettings.scss" 

export const pageConfig: PageConfig = {
  title: "Content Settings",
  subtitle: "Configure post and comment settings",
  sidebarItem: "content",
}

interface LimitSetting {
  count: number
  hours: number
}

interface ContentSettingsModel {
  titleLengthMin: number
  titleLengthMax: number
  descriptionLengthMin: number
  descriptionLengthMax: number
  maxImagesPerPost: number
  maxImagesPerComment: number
  postLimits: Record<string, LimitSetting>
  commentLimits: Record<string, LimitSetting>
  postingDisabledFor: string[]
  commentingDisabledFor: string[]
  postingGloballyDisabled: boolean
  commentingGloballyDisabled: boolean
  reportingGloballyDisabled: boolean
  reportLimitsPerDay: number
}

const ContentSettingsPage: React.FC = () => {
  const fider = useFider()
  
  const defaultSettings: ContentSettingsModel = {
    titleLengthMin: 15,
    titleLengthMax: 100,
    descriptionLengthMin: 150,
    descriptionLengthMax: 1000,
    maxImagesPerPost: 3,
    maxImagesPerComment: 2,
    postLimits: {},
    commentLimits: {},
    postingDisabledFor: [],
    commentingDisabledFor: [],
    postingGloballyDisabled: false,
    commentingGloballyDisabled: false,
    reportingGloballyDisabled: false,
    reportLimitsPerDay: 10
  }
  
  const [settings, setSettings] = useState<ContentSettingsModel>(() => {
    const stored = fider.session.tenant.generalSettings as ContentSettingsModel | null
    if (!stored) return defaultSettings
    return {
      ...defaultSettings,
      ...stored,
    }
  })
  const [error, setError] = useState<Failure | undefined>(undefined)
  const [activeTab, setActiveTab] = useState<'global' | 'post' | 'comment' | 'report'>('global')

  const roles = (fider.session.props.roles as string[]) || []
  
  const canEdit = (fider.session.user.isAdministrator || fider.session.user.isCollaborator)

  const handleSave = async (e: ButtonClickEvent) => {
    const result = await actions.updateGeneralSettings({ settings })
    if (result.ok) {
      e.preventEnable()
      location.href = `/admin/content-settings`
    } else if (result.error) {
      setError(result.error)
    }
  }

  const updateSetting = (key: keyof ContentSettingsModel, value: any) => {
    setSettings({
      ...settings,
      [key]: value
    })
  }

  const updatePostLimit = (role: string, field: keyof LimitSetting, value: number) => {
    const currentLimits = settings.postLimits || {}
    const currentRoleLimit = currentLimits[role] || { count: 0, hours: 24 }
    
    setSettings({
      ...settings,
      postLimits: {
        ...currentLimits,
        [role]: {
          ...currentRoleLimit,
          [field]: value
        }
      }
    })
  }

  const updateCommentLimit = (role: string, field: keyof LimitSetting, value: number) => {
    const currentLimits = settings.commentLimits || {}
    const currentRoleLimit = currentLimits[role] || { count: 0, hours: 24 }
    
    setSettings({
      ...settings,
      commentLimits: {
        ...currentLimits,
        [role]: {
          ...currentRoleLimit,
          [field]: value
        }
      }
    })
  }

  const toggleDisabledRole = (settingKey: 'postingDisabledFor' | 'commentingDisabledFor', role: string) => {
    const currentRoles = settings[settingKey] || []
    const newRoles = currentRoles.includes(role)
      ? currentRoles.filter(r => r !== role)
      : [...currentRoles, role]
    
    setSettings({
      ...settings,
      [settingKey]: newRoles
    })
  }

  const toggleGlobal = (field: 'postingGloballyDisabled' | 'commentingGloballyDisabled') => {
    updateSetting(field, !settings[field])
  }

  const renderTabNav = () => {
    return (
      <div className="settings-tabs-nav">
        <HStack spacing={0} className="mb-0">
          {[
            { key: 'global', label: 'Global Controls' },
            { key: 'post', label: 'Post Settings' },
            { key: 'comment', label: 'Comment Settings' },
            { key: 'report', label: 'Report Settings' }
          ].map(tab => (
            <button 
              key={tab.key}
              type="button"
              className={classSet({
                "settings-tab-button": true,
                "settings-tab-active": activeTab === tab.key
              })}
              onClick={(e) => {
                e.preventDefault();
                setActiveTab(tab.key as 'global' | 'post' | 'comment' | 'report');
              }}
            >
              <span>{tab.label}</span>
            </button>
          ))}
        </HStack>
      </div>
    )
  }

  const renderGlobalControls = () => {
    return (
      <div className={classSet({
        "settings-tab-content": true,
        "settings-tab-visible": activeTab === 'global',
        "settings-tab-hidden": activeTab !== 'global'
      })}>
        <div className="global-controls-panel">          
          <div className="settings-toggle-group">
            <Toggle 
              active={settings.postingGloballyDisabled} 
              label="Disable all posts" 
              onToggle={() => toggleGlobal('postingGloballyDisabled')}
              disabled={!canEdit}
            />
            <p className="settings-description">
              When enabled, no one will be able to create new posts on this site.
            </p>
          </div>
          
          <div className="settings-toggle-group">
            <Toggle 
              active={settings.commentingGloballyDisabled} 
              label="Disable all comments" 
              onToggle={() => toggleGlobal('commentingGloballyDisabled')}
              disabled={!canEdit}
            />
            <p className="settings-description">
              When enabled, no one will be able to comment on any posts.
            </p>
          </div>
        </div>
      </div>
    )
  }
  
  const renderPostSettings = () => {
    return (
      <div className={classSet({
        "settings-tab-content": true,
        "settings-tab-visible": activeTab === 'post',
        "settings-tab-hidden": activeTab !== 'post'
      })}>
        <div className="settings-panels-container">
          <CollapsiblePanel title="Post Length Settings" defaultOpen={true}>
            <div className="settings-grid-2col">
              <Input
                field="titleLengthMin"
                label="Minimum Title Length"
                type="number"
                min={1}
                max={100}
                value={settings.titleLengthMin.toString()}
                disabled={!canEdit}
                onChange={(value) => updateSetting('titleLengthMin', parseInt(value))}
              >
                <p className="settings-description">Minimum characters for post titles.</p>
              </Input>

              <Input
                field="titleLengthMax"
                label="Maximum Title Length"
                type="number"
                min={10}
                max={500}
                value={settings.titleLengthMax.toString()}
                disabled={!canEdit}
                onChange={(value) => updateSetting('titleLengthMax', parseInt(value))}
              >
                <p className="settings-description">Maximum characters for post titles.</p>
              </Input>
            </div>

            <div className="settings-grid-2col mt-4">
              <Input
                field="descriptionLengthMin"
                label="Minimum Description Length"
                type="number"
                min={1}
                max={500}
                value={settings.descriptionLengthMin.toString()}
                disabled={!canEdit}
                onChange={(value) => updateSetting('descriptionLengthMin', parseInt(value))}
              >
                <p className="settings-description">Minimum characters for post descriptions.</p>
              </Input>

              <Input
                field="descriptionLengthMax"
                label="Maximum Description Length"
                type="number"
                min={100}
                max={10000}
                value={settings.descriptionLengthMax.toString()}
                disabled={!canEdit}
                onChange={(value) => updateSetting('descriptionLengthMax', parseInt(value))}
              >
                <p className="settings-description">Maximum characters for post descriptions.</p>
              </Input>
            </div>
          </CollapsiblePanel>

          <CollapsiblePanel title="Post Media Settings" defaultOpen={true}>
            <Input
              field="maxImagesPerPost"
              label="Maximum Images per Post"
              type="number"
              min={0}
              max={10}
              value={settings.maxImagesPerPost.toString()}
              disabled={!canEdit}
              onChange={(value) => updateSetting('maxImagesPerPost', parseInt(value))}
            >
              <p className="settings-description">Maximum number of images that can be attached to a post.</p>
            </Input>
          </CollapsiblePanel>

          <CollapsiblePanel title="Post Rate Limits" defaultOpen={false}>            
            <div className="settings-rate-limits-grid">
              {roles.map(role => (
                <div key={`post-limit-${role}`} className="settings-role-card">
                  <h4 className="settings-role-title">{role}</h4>
                  <div className="settings-role-limits">
                    <Input
                      field={`post-limit-${role}-count`}
                      label="Max Posts"
                      type="number"
                      min={0}
                      max={1000}
                      value={(settings.postLimits?.[role]?.count || 0).toString()}
                      disabled={!canEdit}
                      onChange={(value) => updatePostLimit(role, 'count', parseInt(value))}
                    />
                    <Input
                      field={`post-limit-${role}-hours`}
                      label="Time (hours)"
                      type="number"
                      min={1}
                      max={720}
                      value={(settings.postLimits?.[role]?.hours || 24).toString()}
                      disabled={!canEdit || !(settings.postLimits?.[role]?.count > 0)}
                      onChange={(value) => updatePostLimit(role, 'hours', parseInt(value))}
                    />
                  </div>
                </div>
              ))}
            </div>
          </CollapsiblePanel>

          <CollapsiblePanel title="Post Permissions" defaultOpen={false}>            
            <div className="settings-permissions-grid">
              {roles.map(role => (
                <div key={`posting-disabled-${role}`} className="settings-permission-item">
                  <label className="settings-checkbox">
                    <input
                      type="checkbox"
                      checked={(settings.postingDisabledFor || []).includes(role)}
                      disabled={!canEdit}
                      onChange={() => toggleDisabledRole('postingDisabledFor', role)}
                    />
                    <span>Disable posting for {role}</span>
                  </label>
                </div>
              ))}
            </div>
          </CollapsiblePanel>
        </div>
      </div>
    )
  }

  const renderCommentSettings = () => {
    return (
      <div className={classSet({
        "settings-tab-content": true,
        "settings-tab-visible": activeTab === 'comment',
        "settings-tab-hidden": activeTab !== 'comment'
      })}>
        <div className="settings-panels-container">
          <CollapsiblePanel title="Comment Media Settings" defaultOpen={true}>
            <Input
              field="maxImagesPerComment"
              label="Maximum Images per Comment"
              type="number"
              min={0}
              max={10}
              value={settings.maxImagesPerComment.toString()}
              disabled={!canEdit}
              onChange={(value) => updateSetting('maxImagesPerComment', parseInt(value))}
            >
              <p className="settings-description">Maximum number of images that can be attached to a comment.</p>
            </Input>
          </CollapsiblePanel>

          <CollapsiblePanel title="Comment Rate Limits" defaultOpen={false}>            
            <div className="settings-rate-limits-grid">
              {roles.map(role => (
                <div key={`comment-limit-${role}`} className="settings-role-card">
                  <h4 className="settings-role-title">{role}</h4>
                  <div className="settings-role-limits">
                    <Input
                      field={`comment-limit-${role}-count`}
                      label="Max Comments"
                      type="number"
                      min={0}
                      max={1000}
                      value={(settings.commentLimits?.[role]?.count || 0).toString()}
                      disabled={!canEdit}
                      onChange={(value) => updateCommentLimit(role, 'count', parseInt(value))}
                    />
                    <Input
                      field={`comment-limit-${role}-hours`}
                      label="Time (hours)"
                      type="number"
                      min={1}
                      max={720}
                      value={(settings.commentLimits?.[role]?.hours || 24).toString()}
                      disabled={!canEdit || !(settings.commentLimits?.[role]?.count > 0)}
                      onChange={(value) => updateCommentLimit(role, 'hours', parseInt(value))}
                    />
                  </div>
                </div>
              ))}
            </div>
          </CollapsiblePanel>

          <CollapsiblePanel title="Comment Permissions" defaultOpen={false}>            
            <div className="settings-permissions-grid">
              {roles.map(role => (
                <div key={`commenting-disabled-${role}`} className="settings-permission-item">
                  <label className="settings-checkbox">
                    <input
                      type="checkbox"
                      checked={(settings.commentingDisabledFor || []).includes(role)}
                      disabled={!canEdit}
                      onChange={() => toggleDisabledRole('commentingDisabledFor', role)}
                    />
                    <span>Disable commenting for {role}</span>
                  </label>
                </div>
              ))}
            </div>
          </CollapsiblePanel>
        </div>
      </div>
    )
  }

  const renderReportSettings = () => {
    return (
      <div className={classSet({
        "settings-tab-content": true,
        "settings-tab-visible": activeTab === 'report',
        "settings-tab-hidden": activeTab !== 'report'
      })}>
        <div className="settings-panels-container">
          <CollapsiblePanel title="Report Controls" defaultOpen={true}>
            <div className="settings-toggle-group">
              <Toggle 
                active={settings.reportingGloballyDisabled} 
                label="Disable all reports" 
                onToggle={() => updateSetting('reportingGloballyDisabled', !settings.reportingGloballyDisabled)}
                disabled={!canEdit}
              />
              <p className="settings-description">
                When enabled, users will not be able to submit new reports.
              </p>
            </div>
          </CollapsiblePanel>

          <CollapsiblePanel title="Report Limits" defaultOpen={true}>
            <Input
              field="reportLimitsPerDay"
              label="Maximum Reports per User per Day"
              type="number"
              min={1}
              max={100}
              value={(settings.reportLimitsPerDay || 10).toString()}
              disabled={!canEdit}
              onChange={(value) => updateSetting('reportLimitsPerDay', parseInt(value))}
            >
              <p className="settings-description">
                Maximum number of reports a single user can submit per day. Set to 0 for unlimited.
              </p>
            </Input>
          </CollapsiblePanel>
        </div>
      </div>
    )
  }

  return (
    <Form error={error}>
      <div className="settings-container">
        {renderTabNav()}
        
        <div className="settings-content">
          {renderGlobalControls()}
          {renderPostSettings()}
          {renderCommentSettings()}
          {renderReportSettings()}
        </div>

        <div className="settings-actions">
          <Button disabled={!canEdit} variant="primary" onClick={handleSave}>
            Save Settings
          </Button>
        </div>
      </div>
    </Form>
  )
}

export default ContentSettingsPage
