package rutracker

type ItemSize struct {
	Size float64 `json:"size"`
	Unit string  `json:"unit"`
}
type RutrackerSearchItem struct {
	TopicID     int 		`json:"topic_id"`
	URL         string		`json:"url"`
	DownloadURL string		`json:"download_url"`
	Title       string		`json:"title"`
	Seeders     int			`json:"seeders"`
	Size        ItemSize	`json:"size"`
}
