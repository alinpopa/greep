package main

import (
	"flag"
	"fmt"
	"github.com/alinpopa/greep/data"
	"golang.org/x/net/html"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func startHeartBeat(ec *data.ExecutionContext) {
	ec.Wg.Add(1)
	heartbeat := time.NewTicker(2 * time.Second)
	go func() {
		for {
			<-heartbeat.C
		}
	}()
}

func stopHeartBeat(ec *data.ExecutionContext) {
	ec.Wg.Done()
}

func parseHref(rawUrl string, href string) string {
	url, _ := neturl.Parse(rawUrl)
	for {
		if strings.HasPrefix(href, "//") {
			newUrl := strings.TrimPrefix(href, "//")
			u, _ := neturl.Parse(newUrl)
			if u.Host == url.Host {
				return newUrl
			}
			return ""
		} else if strings.HasPrefix(href, "/") {
			return url.Scheme + "://" + url.Host + href
		} else if strings.HasPrefix(href, "#") {
			return ""
		} else {
			maybeUrl, _ := neturl.Parse(href)
			if maybeUrl.Host == url.Host {
				return href
			} else if maybeUrl.Host == "" && strings.HasSuffix(rawUrl, "/") {
				return rawUrl + href
			} else if maybeUrl.Host == "" {
				return rawUrl + "/" + href
			} else {
				return ""
			}
		}
	}
}

func getPage(url string) []string {
	resp, err := http.Get(url)
	if err != nil {
		return []string{}
	}
	body := resp.Body
	defer body.Close()
	tokenizer := html.NewTokenizer(body)
	links := []string{}
	uniqueLinks := make(map[string]bool)
	for {
		currentToken := tokenizer.Next()
		if currentToken == html.ErrorToken {
			break
		} else if currentToken == html.StartTagToken {
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, a := range token.Attr {
					if a.Key == "href" {
						href := parseHref(url, a.Val)
						if href != "" {
							uniqueLinks[href] = true
						}
					}
				}
			}
		}
	}
	for k := range uniqueLinks {
		links = append(links, k)
	}
	return links
}

func dispatcher(broker *data.Broker, initLink string) {
	parsedlinks := make(map[string]bool)
	parsedlinks[initLink] = true
	readyLinks := []string{initLink}
	for {
		select {
		case w := <-broker.Workers:
			resultLinks := w.Links
			for _, link := range resultLinks {
				if !parsedlinks[link] {
					parsedlinks[link] = true
					readyLinks = append(readyLinks, link)
				}
			}
			if len(readyLinks) != 0 {
				link := readyLinks[0]
				readyLinks = readyLinks[1:]
				w.Work <- link
			} else {
				w.Ready <- true
			}
		}
	}
}

func worker(availableWorkers chan<- data.Worker, name string) {
	self := data.Worker{
		Work:  make(chan string),
		Links: []string{},
		Name:  name,
		Ready: make(chan bool),
	}
	availableWorkers <- self
	for {
		select {
		case link := <-self.Work:
			fmt.Println(self.Name, ": Got link", link)
			self.Links = append(getPage(link), link)
			availableWorkers <- self
		case ready := <-self.Ready:
			if ready {
				availableWorkers <- self
			}
		}
	}
}

func main() {
	urlFlag := flag.String("url", "", "Initial url.")
	flag.Parse()
	if *urlFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	_, err := neturl.Parse(*urlFlag)
	if err != nil {
		fmt.Println(err, *urlFlag)
		flag.PrintDefaults()
		os.Exit(1)
	}
	broker := &data.Broker{
		Status:  make(chan string),
		Workers: make(chan data.Worker),
	}
	go worker(broker.Workers, "worker1")
	go worker(broker.Workers, "worker2")
	go dispatcher(broker, *urlFlag)
	ec := &data.ExecutionContext{Wg: &sync.WaitGroup{}}
	startHeartBeat(ec)
	ec.Wg.Wait()
	fmt.Println("Done")
}
