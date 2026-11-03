package rutracker

import (
	"strconv"
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

func row(topicID, title, dl, seeders string) string {
	r := `<tr class="hl-tr"><td><a data-topic_id="` + topicID + `" class="med tLink" href="viewtopic.php?t=` + topicID + `">` + title + `</a></td>`
	if dl != "" {
		r += `<td><a class="small tr-dl" href="` + dl + `">1.46&nbsp;GB &#8595;</a></td>`
	}
	if seeders != "" {
		r += `<td><b class="seedmed">` + seeders + `</b></td>`
	}
	return r + `</tr>`
}

func parse(t *testing.T, page string) []RutrackerSearchItem {
	t.Helper()
	body, err := charmap.Windows1251.NewEncoder().Bytes([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	items, err := ParseSearchItems(&body)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// F18: a row without a download link (or seeders) is skipped on its own
// instead of borrowing the next row's link, size and seeders.
func TestParseSearchItemsKeepsRowsApart(t *testing.T) {
	page := `<table>` +
		row("111", "Row A no dl", "", "") +
		row("222", "Row B", "dl.php?t=222", "7") +
		row("333", "Row C no seeders", "dl.php?t=333", "") +
		row("444", "Row D", "dl.php?t=444", "3") + `</table>`
	items := parse(t, page)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want rows B and D only", items)
	}
	for _, it := range items {
		if want := "dl.php?t=" + strconv.Itoa(it.TopicID); it.DownloadURL != want {
			t.Errorf("topic %d (%q) got download %q, want %q", it.TopicID, it.Title, it.DownloadURL, want)
		}
	}
	if items[0].Title != "Row B" || items[0].Seeders != 7 || items[1].Title != "Row D" || items[1].Seeders != 3 {
		t.Errorf("items = %+v", items)
	}
}

// A row whose download link names another topic is dropped.
func TestParseSearchItemsDropsMismatchedDownload(t *testing.T) {
	if items := parse(t, `<table>`+row("555", "Odd", "dl.php?t=556", "1")+`</table>`); len(items) != 0 {
		t.Fatalf("items = %+v, want none", items)
	}
}
