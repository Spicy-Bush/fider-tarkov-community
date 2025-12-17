package cmd

type SaveNavigationLinks struct {
	Links []struct {
		Title        string `json:"title"`
		URL          string `json:"url"`
		DisplayOrder int    `json:"displayOrder"`
		Location     string `json:"location"`
	} `json:"links"`
}

