package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"github.com/gocarina/gocsv"
	"github.com/gocolly/colly"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type quest struct {
	ID          string `csv:"id"`
	Name        string `csv:"name"`
	Extra       string `csv:"preamble"`
	Description string `csv:"description"`
	Solution    string `csv:"solution"`
	Extra2      string `csv:"postscript"`
}

func _extractParts(paragraph string, q *quest) {
	splitted := strings.Split(paragraph, "<br/>")
	for i, v := range splitted {
		splitted[i] = strings.TrimSpace(strings.ReplaceAll(v, "\n", ""))
	}
	var reSplitted []string
	for _, v := range splitted {
		if v != "" {
			reSplitted = append(reSplitted, v)
		}
	}
	cases := []string{"Zadanie obowiązuje", "Opis:", "Solucja:"}
	for i, v1 := range cases {
		partFound := false
		for _, v2 := range reSplitted {
			if strings.HasPrefix(v2, v1) {
				partFound = true
				if i > 0 {
					continue
				}
			}
			if partFound {
				if strings.HasPrefix(v2, cases[1]) || strings.HasPrefix(v2, cases[2]) {
					break
				}
				switch i {
				case 0:
					q.Extra += v2
					q.Extra += "\n"
				case 1:
					q.Description += v2
					q.Description += "\n"
				case 2:
					q.Solution += v2
					q.Solution += "\n"
				}
			}
		}
	}
}

func extractParts(paragraph string, q *quest) {
	extraSplit := strings.SplitN(paragraph, "Opis:", 2)
	var toDescSplit string
	if len(extraSplit) == 2 {
		q.Extra = extraSplit[0]
		toDescSplit = extraSplit[1]
	} else {
		toDescSplit = extraSplit[0]
	}

	descriptionSplit := strings.SplitN(toDescSplit, "Solucja:", 2)
	var toSolutionSplit string
	if len(descriptionSplit) == 2 {
		q.Description = descriptionSplit[0]
		toSolutionSplit = descriptionSplit[1]
	} else {
		toSolutionSplit = descriptionSplit[0]
	}

	solutionSplit := strings.SplitN(toSolutionSplit, "Dodatkowo:", 2)
	if len(solutionSplit) == 2 {
		q.Solution = solutionSplit[0]
		q.Extra2 = solutionSplit[1]
	} else {
		q.Solution = solutionSplit[0]
	}
}

func decodeIntoUTF8(s string) string {
	decBytes, err := ioutil.ReadAll(transform.NewReader(strings.NewReader(s), charmap.ISO8859_2.NewDecoder()))
	if err != nil {
		log.Fatalln(err)
	}
	return string(decBytes)
}

func sanitizeField(s string) string {
	return strings.TrimSpace(
		strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						html.UnescapeString(
							decodeIntoUTF8(s)), "<i>", ""), "</i>", ""), "<br/>", ""), "\n", " "))
}

func sanitize(q *quest) {
	q.Extra = sanitizeField(q.Extra)
	q.Description = sanitizeField(q.Description)
	q.Solution = sanitizeField(q.Solution)
	q.Extra2 = sanitizeField(q.Extra2)
}

func main() {
	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL)
	})

	var quests []quest

	internalIdx := 0
	c.OnHTML("b", func(e1 *colly.HTMLElement) {
		e1.ForEach("a", func(idx1 int, e2 *colly.HTMLElement) {
			if e2.Attr("name") == "" {
				return
			}
			if len(quests)-1 < internalIdx {
				quests = append(quests, quest{})
			}
			quests[internalIdx].ID = strings.TrimSuffix(e2.Attr("name"), ".")
			e2.ForEach("u", func(idx2 int, e3 *colly.HTMLElement) {
				quests[internalIdx].Name = decodeIntoUTF8(e3.Text)
			})
			internalIdx++
		})
	})

	c.OnHTML("td", func(e1 *colly.HTMLElement) {
		if e1.Index > 0 {
			return
		}
		html, err := e1.DOM.Html()
		if err != nil {
			log.Fatalln(err)
		}
		questPart := strings.SplitN(html, "<!-- NL2BR true //-->", 2)
		questBody := strings.SplitN(questPart[1], "<p>", 2)
		extractParts(questBody[0], &quests[0])
	})

	c.OnHTML("td", func(e1 *colly.HTMLElement) {
		e1.ForEach("p", func(idx1 int, e2 *colly.HTMLElement) {
			html, err := e2.DOM.Html()
			if err != nil {
				log.Fatalln(err)

			}
			questPart := strings.SplitN(html, "<!-- NL2BR true //-->", 2)
			if len(questPart) < 2 {
				return
			}
			e2.ForEach("b", func(idx2 int, e3 *colly.HTMLElement) {
				e3.ForEach("a", func(idx3 int, e4 *colly.HTMLElement) {
					index, err := strconv.Atoi(strings.TrimSuffix(e4.Attr("name"), "."))
					if err != nil {
						log.Fatalln(err)
					}
					extractParts(questPart[1], &quests[index-1])
				})
			})
		})
	})

	chapters := []string{
		"http://www.gothic.phx.pl/gothic/rozdzial1.php",
		"http://www.gothic.phx.pl/gothic/rozdzial2.php",
		"http://www.gothic.phx.pl/gothic/rozdzial3.php",
		"http://www.gothic.phx.pl/gothic/rozdzial4.php",
		"http://www.gothic.phx.pl/gothic/rozdzial5.php",
	}

	gocsv.SetCSVWriter(func(out io.Writer) *gocsv.SafeCSVWriter {
		writer := csv.NewWriter(out)
		writer.Comma = '|'
		return gocsv.NewSafeCSVWriter(writer)
	})

	for i, v := range chapters {
		quests = nil
		internalIdx = 0

		c.Visit(v)

		for i := range quests {
			sanitize(&quests[i])
		}

		csvStr, err := gocsv.MarshalString(&quests)
		if err != nil {
			log.Fatalln(err)
		}

		if err := ioutil.WriteFile(fmt.Sprintf("./chapter_%d.csv", i+1), []byte(csvStr), 0644); err != nil {
			log.Fatalln(err)
		}
	}
}
