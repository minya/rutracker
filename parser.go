package rutracker

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/minya/logger"
	"golang.org/x/text/encoding/charmap"
)

// The results table is read one row at a time, so a field that is missing
// in one row (a topic without a download link or seeders cell) cannot be
// filled from the next row: one regexp over the whole page used to pair
// row A's title with row B's download link, and row B disappeared.
var (
	rowRe     = regexp.MustCompile(`(?s)<tr\s[^>]*>.*?</tr>`)
	topicRe   = regexp.MustCompile(`(?s)<a\sdata-topic_id="(\d+)"[^>]*?href="([^"]+)"[^>]*>(.+?)</a>`)
	dlRe      = regexp.MustCompile(`(?s)<a[^>]+?href="(dl\.php\?t=(\d+))"[^>]*>([^<&]+?)&nbsp;(\S+?)\s&#8595;</a>`)
	seedersRe = regexp.MustCompile(`<b class="[^"]*">(\d+)</b>`)
)

func ParseSearchItems(responseBytes *[]byte) ([]RutrackerSearchItem, error) {
	out, err := charmap.Windows1251.NewDecoder().Bytes(*responseBytes)
	if err != nil {
		logger.Error(err, "cannot decode response bytes")
		return nil, err
	}

	var result []RutrackerSearchItem
	for _, row := range rowRe.FindAllString(string(out), -1) {
		if item, ok := parseRow(row); ok {
			result = append(result, item)
		}
	}
	return result, nil
}

// parseRow reads one result row. A row without a topic link, a download
// link or a seeders count is skipped, and so is one whose download link
// names another topic.
func parseRow(row string) (RutrackerSearchItem, bool) {
	topic := topicRe.FindStringSubmatch(row)
	if topic == nil {
		return RutrackerSearchItem{}, false
	}
	topicID, err := strconv.Atoi(topic[1])
	if err != nil {
		return RutrackerSearchItem{}, false
	}
	dlLoc := dlRe.FindStringSubmatchIndex(row)
	if dlLoc == nil {
		return RutrackerSearchItem{}, false
	}
	dl := dlRe.FindStringSubmatch(row)
	if dl[2] != topic[1] {
		logger.Warn("Skipping search row: topic %s links to download %s", topic[1], dl[1])
		return RutrackerSearchItem{}, false
	}
	seeders := seedersRe.FindStringSubmatch(row[dlLoc[1]:])
	if seeders == nil {
		return RutrackerSearchItem{}, false
	}
	seedCount, err := strconv.Atoi(seeders[1])
	if err != nil {
		logger.Error(err, "Cannot parse seeders. Got %s", seeders[1])
		return RutrackerSearchItem{}, false
	}
	return RutrackerSearchItem{
		TopicID:     topicID,
		DownloadURL: dl[1],
		URL:         html.UnescapeString(topic[2]),
		Title:       plainText(topic[3]),
		Seeders:     seedCount,
		Size: ItemSize{
			Size: parseSize(dl[3]),
			Unit: dl[4],
		},
	}, true
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// plainText turns the topic link's inner HTML into display text: tags such
// as <wbr> are dropped and entities such as &quot; decoded.
func plainText(innerHTML string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(innerHTML, "")))
}

func parseSize(sizeStr string) float64 {
	size, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		logger.Error(err, "Cannot parse size. Got %s", sizeStr)
		return 0
	}
	return size
}
