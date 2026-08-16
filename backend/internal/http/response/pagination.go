package response

type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type PageData struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}
