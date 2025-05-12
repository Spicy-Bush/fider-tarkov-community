import React, { useState } from "react"
import { Input, Avatar, UserName, Icon, Dropdown, Button } from "@fider/components"
import { User, UserRole, UserStatus, VisualRole } from "@fider/models"
import { AdminBasePage } from "../components/AdminBasePage"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
import IconDotsHorizontal from "@fider/assets/images/heroicons-dots-horizontal.svg"
import { actions, Failure, Fider } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { ModerationModal } from "@fider/components/ModerationModal"

interface ManageMembersPageState {
  query: string
  users: User[]
  visibleUsers: User[]
}

interface ManageMembersPageProps {
  users: User[]
}

interface UserListItemProps {
  user: User
  onAction: (actionName: string, user: User) => Promise<void>
}

const UserListItem = (props: UserListItemProps) => {
  const admin = props.user.role === UserRole.Administrator && <span>administrator</span>
  const collaborator = props.user.role === UserRole.Collaborator && <span>collaborator</span>
  const moderator = props.user.role === UserRole.Moderator && <span>moderator</span>
  const helper = props.user.role === UserRole.Helper && <span>helper</span>
  const blocked = props.user.status === UserStatus.Blocked && <span className="text-red-700">blocked</span>
  const [showModModal, setShowModModal] = useState(false)
  const [modAction, setModAction] = useState<'mute' | 'warning' | null>(null)
  const [error, setError] = useState<Failure | undefined>()
  
  const canViewEmails = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator

  const actionSelected = (actionName: string) => () => {
    if (actionName === 'view-profile') {
      window.location.href = `/profile/${props.user.id}`
    } else if (actionName === 'mute' || actionName === 'warning') {
      setModAction(actionName)
      setShowModModal(true)
    } else {
      props.onAction(actionName, props.user)
    }
  }

  const handleModAction = async (data: { reason: string; duration: string }) => {
    let result;
    if (modAction === 'mute') {
      result = await actions.muteUser(props.user.id, {
        reason: data.reason,
        duration: data.duration
      });
    } else if (modAction === 'warning') {
      result = await actions.warnUser(props.user.id, {
        reason: data.reason,
        duration: modAction === 'warning' && data.duration === "24h" ? "0" : data.duration
      });
    } else {
      return;
    }

    if (result.ok) {
      setShowModModal(false)
      setModAction(null)
      props.onAction('refresh', props.user)
    } else if (result.error) {
      setError(result.error)
    }
  }

  return (
    <HStack spacing={4}>
      <HStack spacing={4}>
        <Avatar user={props.user} />
        <VStack spacing={0}>
          <UserName user={props.user} showEmail={canViewEmails} />
          {props.user.providers && props.user.providers.length > 0 && (
            <span className="text-muted">
              Provider: {props.user.providers.map(p => `${p.name}: ${p.uid}`).join(", ")}
            </span>
          )}
          <span className="text-muted">
            {admin} {moderator} {helper} {collaborator}  {blocked}
          </span>
        </VStack>
      </HStack>
      {Fider.session.user.id !== props.user.id && (
        <Dropdown renderHandle={<Icon sprite={IconDotsHorizontal} width="16" height="16" />}>
          <Dropdown.ListItem onClick={actionSelected("view-profile")}>
            View Profile
          </Dropdown.ListItem>
          <Dropdown.Divider />
          {Fider.session.user.isAdministrator && !blocked && (props.user.role !== UserRole.Administrator) && (
            <Dropdown.ListItem onClick={actionSelected("to-administrator")}>
              Promote to Administrator
            </Dropdown.ListItem>
          )}
          {Fider.session.user.isAdministrator && !blocked && (props.user.role !== UserRole.Moderator) && (
            <Dropdown.ListItem onClick={actionSelected("to-moderator")}>
              {props.user.role === UserRole.Administrator ? "Demote to Moderator" : "Promote to Moderator"}
            </Dropdown.ListItem>
          )}
          {Fider.session.user.isAdministrator && !blocked && (props.user.role !== UserRole.Collaborator) && (
            <Dropdown.ListItem onClick={actionSelected("to-collaborator")}>
              {props.user.role === UserRole.Administrator ? "Demote to Collaborator" : "Promote to Collaborator"}
            </Dropdown.ListItem>
          )}
          {Fider.session.user.isAdministrator && !blocked && (props.user.role !== UserRole.Helper) && (
            <Dropdown.ListItem onClick={actionSelected("to-helper")}>
              {(props.user.role === UserRole.Administrator || props.user.role === UserRole.Collaborator || props.user.role === UserRole.Moderator) ? "Demote to Helper" : "Promote to Helper"}
            </Dropdown.ListItem>
          )}
          {Fider.session.user.isAdministrator && !blocked && (props.user.role !== UserRole.Visitor) && (
            <Dropdown.ListItem onClick={actionSelected("to-visitor")}>
              Demote to Visitor
            </Dropdown.ListItem>
          )}
          
          {!blocked && Fider.session.user.isAdministrator && (
            <Dropdown.ListItem onClick={actionSelected("block")}>Block User</Dropdown.ListItem>
          )}
          {!blocked && Fider.session.user.isCollaborator && !Fider.session.user.isAdministrator && 
            (props.user.role === UserRole.Moderator || props.user.role === UserRole.Visitor) && (
            <Dropdown.ListItem onClick={actionSelected("block")}>Block User</Dropdown.ListItem>
          )}

          {!blocked && Fider.session.user.isAdministrator && (
            <>
              <Dropdown.ListItem onClick={actionSelected("mute")}>Mute User</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("warning")}>Warn User</Dropdown.ListItem>
            </>
          )}
          {!blocked && Fider.session.user.isCollaborator && !Fider.session.user.isAdministrator && 
            (props.user.role === UserRole.Moderator || props.user.role === UserRole.Visitor) && (
            <>
              <Dropdown.ListItem onClick={actionSelected("mute")}>Mute User</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("warning")}>Warn User</Dropdown.ListItem>
            </>
          )}
          {!blocked && Fider.session.user.isModerator && !Fider.session.user.isCollaborator && !Fider.session.user.isAdministrator && 
            props.user.role === UserRole.Visitor && (
            <>
              <Dropdown.ListItem onClick={actionSelected("mute")}>Mute User</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("warning")}>Warn User</Dropdown.ListItem>
            </>
          )}

          {!!blocked && (Fider.session.user.isAdministrator || 
            (Fider.session.user.isCollaborator && (props.user.role === UserRole.Moderator || props.user.role === UserRole.Visitor))) && (
            <Dropdown.ListItem onClick={actionSelected("unblock")}>Unblock User</Dropdown.ListItem>
          )}
        
          {(Fider.session.user.isAdministrator || Fider.session.user.isCollaborator) && (
            <>
              <Dropdown.Divider />
              <Dropdown.ListItem onClick={actionSelected("visualrole-none")}>Set Visual Role: None</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-visitor")}>Set Visual Role: Visitor</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-helper")}>Set Visual Role: Helper</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-administrator")}>Set Visual Role: Administrator</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-moderator")}>Set Visual Role: Moderator</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-bsgcrew")}>Set Visual Role: BSG Crew</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-developer")}>Set Visual Role: Developer</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-sherpa")}>Set Visual Role: Sherpa</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-tcstaff")}>Set Visual Role: TC Staff</Dropdown.ListItem>
              <Dropdown.ListItem onClick={actionSelected("visualrole-emissary")}>Set Visual Role: Emissary</Dropdown.ListItem>
            </>
          )}
        </Dropdown>
      )}
      <ModerationModal
        isOpen={showModModal}
        onClose={() => setShowModModal(false)}
        actionType={modAction || 'mute'}
        onSubmit={handleModAction}
        error={error}
      />
    </HStack>
  )
}

export default class ManageMembersPage extends AdminBasePage<ManageMembersPageProps, ManageMembersPageState> {
  public id = "p-admin-members"
  public name = "members"
  public title = "Members"
  public subtitle = "Manage your site administrators and collaborators"

  constructor(props: ManageMembersPageProps) {
    super(props)

    // Sort the users using our custom sort order (see sortByStaff below)
    const users = this.props.users.sort(this.sortByStaff)
    
    this.state = {
      query: "",
      users,
      visibleUsers: users.slice(0, 10),
    }
  }

  
  private showMore = (): void => {
    this.setState({
      visibleUsers: this.state.users.slice(0, this.state.visibleUsers.length + 10),
    })
  }

  private clearSearch = () => {
    this.handleSearchFilterChanged("")
  }

  private memberFilter = (query: string, user: User): boolean => {
    const queryLower = query.toLowerCase();
    const canSearchByEmail = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator;
    
    if (user.name.toLowerCase().indexOf(queryLower) >= 0) {
      return true;
    }
    
    if (canSearchByEmail && user.email && user.email.toLowerCase().indexOf(queryLower) >= 0) {
      return true;
    }

    if (user.providers && user.providers.length > 0) {
      for (const provider of user.providers) {
        if (provider.uid.toLowerCase().indexOf(queryLower) >= 0) {
          return true;
        }
      }
    }
    
    return false;
  }

  private handleSearchFilterChanged = (query: string) => {
    const users = this.props.users
      .filter((x) => this.memberFilter(query, x))
      .sort(this.sortByStaff)
    this.setState({ query, users, visibleUsers: users.slice(0, 10) })
  }

  private handleAction = async (actionName: string, user: User) => {
    const changeRole = async (role: UserRole) => {
      const result = await actions.changeUserRole(user.id, role)
      if (result.ok) {
        const originalUser = this.props.users.find(u => u.id === user.id);
        if (originalUser) {
          originalUser.role = role;
        }
        this.handleSearchFilterChanged(this.state.query);
      }
    }
    
    const changeStatus = async (status: UserStatus) => {
      const action = status === UserStatus.Blocked ? actions.blockUser : actions.unblockUser
      const result = await action(user.id)
      if (result.ok) {
        const originalUser = this.props.users.find(u => u.id === user.id);
        if (originalUser) {
          originalUser.status = status;
        }
        this.handleSearchFilterChanged(this.state.query);
      }
    }
    
    const changeVisualRole = async (visualRole: VisualRole) => {
      try {
        const visualRoleToNumber: Record<string, number> = { "": 0, "Visitor": 1, "Helper": 2,
          "Administrator": 3, "Moderator": 4, "BSGCrew": 5, "Developer": 6,"Sherpa": 7, "TCStaff": 8,
          "Emissary": 9 };
        const response = await fetch(`/_api/admin/visualroles/${visualRoleToNumber[visualRole]}/users`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ userID: user.id }),
        });
        
        if (response.ok) {
          const originalUser = this.props.users.find(u => u.id === user.id);
          if (originalUser) {
            originalUser.visualRole = visualRole;
          }
          
          user.visualRole = visualRole;
          
          this.forceUpdate();
        }
      } catch (error) {
        console.error("Failed to change visual role:", error);
      }
    }

    const actionHandlers: { [key: string]: () => Promise<void> } = {
      "to-administrator": () => changeRole(UserRole.Administrator),
      "to-moderator": () => changeRole(UserRole.Moderator),
      "to-collaborator": () => changeRole(UserRole.Collaborator),
      "to-helper": () => changeRole(UserRole.Helper),
      "to-visitor": () => changeRole(UserRole.Visitor),
      "block": () => changeStatus(UserStatus.Blocked),
      "unblock": () => changeStatus(UserStatus.Active),
      "visualrole-none": () => changeVisualRole(VisualRole.None),
      "visualrole-visitor": () => changeVisualRole(VisualRole.Visitor),
      "visualrole-helper": () => changeVisualRole(VisualRole.Helper),
      "visualrole-administrator": () => changeVisualRole(VisualRole.Administrator),
      "visualrole-moderator": () => changeVisualRole(VisualRole.Moderator),
      "visualrole-bsgcrew": () => changeVisualRole(VisualRole.BSGCrew),
      "visualrole-developer": () => changeVisualRole(VisualRole.Developer),
      "visualrole-sherpa": () => changeVisualRole(VisualRole.Sherpa),
      "visualrole-tcstaff": () => changeVisualRole(VisualRole.TCStaff),
      "visualrole-emissary": () => changeVisualRole(VisualRole.Emissary)
    };

    const handler = actionHandlers[actionName];
    if (handler) {
      await handler();
    }
  }

  private sortByStaff = (left: User, right: User) => {
    const rolePriority: { [key in UserRole]: number } = {
      [UserRole.Administrator]: 1,
      [UserRole.Collaborator]: 2,
      [UserRole.Moderator]: 3,
      [UserRole.Helper]: 4,
      [UserRole.Visitor]: 5,
    }

    if (rolePriority[left.role] === rolePriority[right.role]) {
      return left.name.localeCompare(right.name)
    }
    return rolePriority[left.role] - rolePriority[right.role]
  }

  public content() {
    const canViewEmails = Fider.session.user.isAdministrator || Fider.session.user.isCollaborator;
    const searchPlaceholder = canViewEmails 
      ? "Search for users by name / email / provider id ..."
      : "Search for users by name / provider id ...";

    return (
      <>
        <Input
          field="query"
          icon={this.state.query ? IconX : IconSearch}
          onIconClick={this.state.query ? this.clearSearch : undefined}
          placeholder={searchPlaceholder}
          value={this.state.query}
          onChange={this.handleSearchFilterChanged}
        />
        <div className="p-2">
          <VStack spacing={2} divide={true}>
            {this.state.visibleUsers.map((user) => (
              <UserListItem key={user.id} user={user} onAction={this.handleAction} />
            ))}
          </VStack>
        </div>
        <p className="text-muted pt-4">
          {!this.state.query && (
            <>
              Showing {this.state.visibleUsers.length} of {this.state.users.length} registered users.
            </>
          )}
          {this.state.query && (
            <>
              Showing {this.state.visibleUsers.length} of {this.state.users.length} users matching &apos;{this.state.query}&apos;.
            </>
          )}
          {this.state.visibleUsers.length < this.state.users.length && (
            <Button variant="tertiary" onClick={this.showMore}>
              view more
            </Button>
          )}
        </p>
        {Fider.session.user.isAdministrator && (
        <ul className="text-muted">
          <li>
            <strong>Administrators</strong> have full access to edit and manage content, permissions and all site settings.
          </li>
          <li>
            <strong>Moderators</strong> can moderate discussions and manage community interactions. They can warn and mute users. They cannot view user email addresses.
          </li>
          <li>
            <strong>Collaborators</strong> can edit and manage content, set visual roles, and warn/mute/block users, but not change user roles.
          </li>
          <li>
            <strong>Blocked</strong> users are unable to log into this site.
          </li>
          <li>
            <strong>Visual Roles</strong> are display-only roles that affect a user's appearance in the UI.
          </li>
        </ul>
        )}
      </>
    )
  }
}
