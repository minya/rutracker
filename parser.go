package rutracker

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/minya/logger"
	"golang.org/x/text/encoding/charmap"
)

var pattern = `<tr\s[.\s\S]+?<a\sdata-topic_id="(\d+)".+?href="(.+?)">(.+?)<\/a>[.\s\S]+?<a.+?href="(dl\.php\?t=\d+)">(.+)&nbsp;(.+?)\s&#8595;<\/a>[.\s\S]+?<b class=".+?">(.+?)<\/b>[.\s\S]+?<\/tr>`

var re = regexp.MustCompile(pattern)

func ParseSearchItems(responseBytes *[]byte) ([]RutrackerSearchItem, error) {
	out, err := charmap.Windows1251.NewDecoder().Bytes(*responseBytes)
	if err != nil {
		logger.Error(err, "cannot decode response bytes")
		return nil, err
	}
	searchResultsHtml := string(out)

	match := re.FindAllStringSubmatch(searchResultsHtml, -1)

	var result []RutrackerSearchItem

	for i := range match {
		group := match[i]
		topicID, err := strconv.Atoi(group[1])
		if err != nil {
			continue
		}
		seeders, err := strconv.Atoi(group[7])
		if err != nil {
			logger.Error(err, "Cannot parse seeders. Got %s", group[7])
			continue
		}
		item := RutrackerSearchItem{
			TopicID:     topicID,
			DownloadURL: group[4],
			URL:         html.UnescapeString(group[2]),
			Title:       plainText(group[3]),
			Seeders:     seeders,
			Size: ItemSize{
				Size: parseSize(group[5]),
				Unit: group[6],
			},
		}
		result = append(result, item)
	}
	return result, nil
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

