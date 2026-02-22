package bitrix

type ListResponse[T any] struct {
	Result []T  `json:"result"`
	Next   *int `json:"next,omitempty"`
	Total  *int `json:"total,omitempty"`
}

type Status struct {
	EntityID   string `json:"ENTITY_ID"`
	StatusID   string `json:"STATUS_ID"`
	Name       string `json:"NAME"`
	CategoryID string `json:"CATEGORY_ID"`
}

type DealCategory struct {
	ID   string `json:"ID"`
	Name string `json:"NAME"`
}

type User struct {
	ID         string `json:"ID"`
	Name       string `json:"NAME"`
	LastName   string `json:"LAST_NAME"`
	SecondName string `json:"SECOND_NAME"`
}

type UsersResponse struct {
	Result []User `json:"result"`
}

type DealUserFieldListItem struct {
	ID    string `json:"ID"`
	Value string `json:"VALUE"`
}

type DealUserField struct {
	FieldName string                  `json:"FIELD_NAME"`
	List      []DealUserFieldListItem `json:"LIST"`
}

type Deal struct {
	ID                 string `json:"ID"`
	CategoryID         string `json:"CATEGORY_ID"`
	StageID            string `json:"STAGE_ID"`
	AssignedByID       string `json:"ASSIGNED_BY_ID"`
	SourceID           string `json:"SOURCE_ID"`
	DateCreate         string `json:"DATE_CREATE"`
	DateModify         string `json:"DATE_MODIFY"`
	UTMSource          string `json:"UTM_SOURCE"`
	UTMCampaign        string `json:"UTM_CAMPAIGN"`
	UFClientType       string `json:"UF_CRM_1647265424537"`
	UFCoopType         string `json:"UF_CRM_1740477560309"`
	UFCRM1650279712660 string `json:"UF_CRM_1650279712660"`
	UFCRM1699841388494 string `json:"UF_CRM_1699841388494"`
	UFCRM1699863367472 string `json:"UF_CRM_1699863367472"`
	UFCRM1752578793696 string `json:"UF_CRM_1752578793696"`
	UFCRM1753169789836 string `json:"UF_CRM_1753169789836"`
	UFCRM1771313479555 string `json:"UF_CRM_1771313479555"`
}
