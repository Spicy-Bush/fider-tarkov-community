package entity

type PagePermissions struct {
	Edit      bool `json:"edit"`
	Delete    bool `json:"delete"`
	React     bool `json:"react"`
	Subscribe bool `json:"subscribe"`
}

func (page *Page) AllowedActions(user *User, tenant *Tenant) PagePermissions {
	discussion := PageDiscussion(page)
	manage := Can(user, tenant, ManagePages)
	return PagePermissions{
		Edit:      manage,
		Delete:    manage,
		React:     discussion.Permissions(user, tenant).React,
		Subscribe: discussion.CanSubscribeToPage(user, tenant),
	}
}

func (discussion *Discussion) CanSubscribeToPage(user *User, tenant *Tenant) bool {
	return canAct(user, tenant) && discussion.CanView(user)
}
