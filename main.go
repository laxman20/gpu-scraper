package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"gopkg.in/toast.v1"
)

type SearchConfig struct {
	url           string                        // url of page to watch
	resultsSel    string                        // selector for html element containing all products
	linkSel       string                        // selector for element containing href with product url
	stockSel      string                        // selector for element to be used for determining availability
	hasStock      func(*goquery.Selection) bool // func that takes stockSel and returns true if product has stock
	linkTransform func(string) string           // used to clean up link if needed (remove tracking params)
}

type ProductInfo struct {
	link    string
	title   string
	inStock bool
}

var items map[string]struct{}
var sidReg = regexp.MustCompile("&sid=.+$")
var refReg = regexp.MustCompile("/ref=.+$")
var gpuReg = regexp.MustCompile("RTX (3060 Ti|3070|3070 Ti|3080)")

func main() {
	ch := make(chan ProductInfo)

	go visit(SearchConfig{
		url:        "https://www.newegg.ca/p/pl?N=100007708%20601359415%20601357250%20601357247%208000&Order=1",
		resultsSel: ".items-grid-view",
		linkSel:    ".item-container > .item-info > .item-title",
		stockSel:   ".item-promo",
		hasStock: func(s *goquery.Selection) bool {
			return strings.TrimSpace(s.Text()) != "OUT OF STOCK"
		},
	}, ch)

	go visit(SearchConfig{
		url:        "https://www.canadacomputers.com/index.php?cPath=43&sf=:3_5,3_6,3_7&mfr=&pr=",
		resultsSel: "div[id='product-list']",
		linkSel:    ".productTemplate_title > a",
		stockSel:   "button",
		hasStock: func(s *goquery.Selection) bool {
			return strings.TrimSpace(s.Text()) == "Add to Cart"
		},
		linkTransform: func(link string) string {
			return sidReg.ReplaceAllString(link, "")
		},
	}, ch)

	go visit(SearchConfig{
		url:        "https://www.amazon.ca/s?i=electronics&bbn=677243011&rh=n%3A677243011%2Cp_6%3AA3DWYIK6Y9EEQB%2Cp_36%3A60000-&dc&qid=1631712885&rnid=12035759011&ref=sr_nr_p_36_5",
		resultsSel: ".s-search-results",
		linkSel:    "h2 > a",
		stockSel:   "h2 > a > span",
		hasStock: func(s *goquery.Selection) bool {
			return gpuReg.FindString(s.Text()) != ""
		},
		linkTransform: func(link string) string {
			return "https://www.amazon.ca" + refReg.ReplaceAllString(link, "")
		},
	}, ch)

	items = make(map[string]struct{})
	for {
		p := <-ch
		_, exists := items[p.link]
		if p.inStock {
			if !exists {
				items[p.link] = struct{}{}
				notify(p.link, p.title)
			}
		} else if exists {
			delete(items, p.link)
		}
	}
}

func visit(config SearchConfig, ch chan ProductInfo) {
	for {
		c := colly.NewCollector()
		extensions.RandomUserAgent(c)

		c.OnHTML(config.resultsSel, func(e *colly.HTMLElement) {
			productElements := e.DOM.Children()
			productElements.Each(func(i int, s *goquery.Selection) {
				productLink := s.Find(config.linkSel)
				link, _ := productLink.Attr("href")
				if config.linkTransform != nil {
					link = config.linkTransform(link)
				}
				stockIndicator := s.Find(config.stockSel)
				hasStock := config.hasStock(stockIndicator)
				ch <- ProductInfo{
					link:    link,
					title:   productLink.Text(),
					inStock: hasStock,
				}
			})
		})

		c.OnError(func(r *colly.Response, err error) {
			fmt.Println("Request: ", r.Request.URL, " failed with response: ", r, "\nError: ", err)
		})

		c.Visit(config.url)
		c.Wait()
		time.Sleep(5 * time.Second)
	}
}

func notify(url string, title string) {
	title = fmt.Sprintf("%.80s", title)
	fmt.Println(time.Now().Format("Jan 2, 3:04PM"))
	fmt.Println(title)
	fmt.Printf("%s\n\n", url)
	notification := toast.Notification{
		AppID:               "GPU Scraper",
		Title:               title,
		ActivationArguments: strings.ReplaceAll(url, "&", "&amp;"),
	}
	err := notification.Push()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}
