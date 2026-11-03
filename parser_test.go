package rutracker

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

// F61: URL is the topic link, and Title is plain text without HTML
// entities or tags.
func TestParseSearchItemsFields(t *testing.T) {
	page := `<table><tr class="hl-tr"><td><a data-topic_id="123" class="med tLink" href="viewtopic.php?t=123&amp;start=0">Фильм &quot;Тест&quot; [2020, <wbr>BDRip]</a></td>` +
		`<td><a class="small tr-dl" href="dl.php?t=123">1.5&nbsp;GB &#8595;</a></td><td><b class="seedmed">42</b></td></tr></table>`
	body, err := charmap.Windows1251.NewEncoder().Bytes([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	items, err := ParseSearchItems(&body)
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %+v, err = %v", items, err)
	}
	it := items[0]
	if it.URL != "viewtopic.php?t=123&start=0" {
		t.Errorf("URL = %q, want the topic link", it.URL)
	}
	if it.Title != `Фильм "Тест" [2020, BDRip]` {
		t.Errorf("Title = %q, want plain text", it.Title)
	}
	if it.TopicID != 123 || it.DownloadURL != "dl.php?t=123" || it.Seeders != 42 || it.Size.Size != 1.5 || it.Size.Unit != "GB" {
		t.Errorf("other fields = %+v", it)
	}
}
