package models

type JikanSearchResponse struct {
	Data       []AnimeData `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

type AnimeData struct {
	MalId    int     `json:"mal_id"`
	Title    string  `json:"title"`
	Score    float64 `json:"score"`
	Episodes int     `json:"episodes"`
	Status   string  `json:"status"`
	Synopsis string  `json:"synopsis"`
	Images   Images  `json:"images"`
	Genres   []Genre `json:"genres"`
	Year     int     `json:"year"`
	Type     string  `json:"type"`
}

type Images struct {
	JPG ImageURL `json:"jpg"`
}

type ImageURL struct {
	ImageURL string `json:"image_url"`
}

type Genre struct {
	Name string `json:"name"`
}

type Pagination struct {
	HasNextPage bool `json:"has_next_page"`
	Items       struct {
		Count int `json:"count"`
		Total int `json:"total"`
	} `json:"items"`
}
