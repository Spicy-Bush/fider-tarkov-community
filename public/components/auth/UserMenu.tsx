import React from "react"
import { useFider } from "@fider/hooks/use-fider"
import { Avatar } from "@fider/components/common/Avatar"
import { Dropdown } from "@fider/components/common/Dropdown"
import { Trans } from "@lingui/react/macro"

export const UserMenu = () => {
  const fider = useFider()
  const permissions = fider.session.permissions

  return (
    <div className="c-menu-user shrink-0">
      <Dropdown position="left" renderHandle={<Avatar user={fider.session.user} clickable={false} />}>
        <div className="p-2 text-medium uppercase">{fider.session.user.name}</div>
        <Dropdown.ListItem href="/profile#settings">
          <Trans id="menu.mysettings">My Settings</Trans>
        </Dropdown.ListItem>
        <Dropdown.ListItem href="/profile">
          <Trans id="menu.myprofile">My Profile</Trans>
        </Dropdown.ListItem>
        <Dropdown.Divider />

        {(permissions.manageQueue || permissions.manageMembers) && (
          <>
            <div className="p-2 text-medium uppercase">
              <Trans id="menu.moderation">Moderation</Trans>
            </div>
            {permissions.manageQueue && (
              <Dropdown.ListItem href="/admin/queue">
                <Trans id="menu.postqueue">Post Queue</Trans>
              </Dropdown.ListItem>
            )}
            {permissions.manageMembers && (
              <Dropdown.ListItem href="/admin/members">
                <Trans id="menu.members">Member List</Trans>
              </Dropdown.ListItem>
            )}
            <Dropdown.Divider />
          </>
        )}
        {permissions.readSettings && (
          <>
            <div className="p-2 text-medium uppercase">
              <Trans id="menu.administration">Administration</Trans>
            </div>
            <Dropdown.ListItem href="/admin">
              <Trans id="menu.sitesettings">Site Settings</Trans>
            </Dropdown.ListItem>
            <Dropdown.Divider />
          </>
        )}
        <Dropdown.ListItem href="/signout?redirect=/">
          <Trans id="menu.signout">Sign out</Trans>
        </Dropdown.ListItem>
      </Dropdown>
    </div>
  )
}

