package main

import (
    "fmt"
    "github.com/darkhelmet/env"
    rss "github.com/jteeuwen/go-pkg-rss"
    T "html/template"
    "log"
    "net/http"
    "sort"
    "time"
)

var (
    Feeds = []string{
        "http://comicsrss.herokuapp.com/cad",
        "http://feeds.feedburner.com/thedoghousediaries/feed",
        "http://comicsrss.herokuapp.com/cyanide",
        "http://www.questionablecontent.net/QCRSS.xml",
        "http://twitterthecomic.tumblr.com/rss",
        "http://www.xkcd.com/rss.xml",
    }
    Feed = T.Must(T.New("rss").Funcs(T.FuncMap{
        "Safe":  func(s string) T.HTML { return T.HTML(s) },
        "CDATA": func(s string) T.HTML { return T.HTML(fmt.Sprintf("<![CDATA[\n%s\n]]>", s)) },
    }).Parse(`{{"<?xml version=\"1.0\" encoding=\"UTF-8\"?>" | Safe}}
<rss version="2.0">
    <channel>
        <title>RSS! SMASH!</title>
        <link>http://rss-smash.herokuapp.com/rss.xml</link>
        <description>An RSS mashup of all my comics</description>
        {{range .}}
        <item>
            <title>{{.Title}}</title>
            <link>{{.Link}}</link>
            <description>
                {{.Description | CDATA}}
            </description>
            <guid>{{.Guid}}</guid>
        </item>
        {{end}}
    </channel>
</rss>
`))
)

type Item struct {
    Title, Link, Description, Guid string
    PubDate                        time.Time
}

type SortedItems []*Item

func (si SortedItems) Len() int {
    return len(si)
}

func (si SortedItems) Less(i, j int) bool {
    return si[i].PubDate.After(si[j].PubDate)
}

func (si SortedItems) Swap(i, j int) {
    si[i], si[j] = si[j], si[i]
}

func fetchFeedItems(url string, items chan *rss.Item, done chan string) {
    channelHandler := func(f *rss.Feed, newchannels []*rss.Channel) {
        for _, channel := range newchannels {
            log.Printf("got new channel %s with %d items", channel.Title, len(channel.Items))
            for _, item := range channel.Items {
                items <- item
            }
            done <- channel.Title
        }
    }

    go func() {
        feed := rss.New(5, true, channelHandler, nil)
        err := feed.Fetch(url, nil)
        if err != nil {
            log.Printf("failed fetching %s: %s", url, err)
        }
    }()
}

func fetchAllFeedItems(urls []string) (items SortedItems) {
    ichan := make(chan *rss.Item)
    count := len(urls)
    done := make(chan string)

    for _, url := range Feeds {
        fetchFeedItems(url, ichan, done)
    }

    for {
        if count == 0 {
            break
        }

        select {
        case item := <-ichan:
            pubdate, err := time.Parse(time.RFC1123Z, item.PubDate)
            if err == nil {
                items = append(items, &Item{
                    Title:       item.Title,
                    Link:        item.Links[0].Href,
                    Description: item.Description,
                    Guid:        item.Guid,
                    PubDate:     pubdate.UTC(),
                })
            }
        case title := <-done:
            log.Printf("finished %s", title)
            count--
        }
    }

    sort.Sort(items)

    return
}

func rssHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/rss+xml")
    w.WriteHeader(200)
    items := fetchAllFeedItems(Feeds)
    log.Printf("got %d items", len(items))
    Feed.Execute(w, items)
}

func main() {
    port := env.StringDefault("PORT", "5000")
    http.HandleFunc("/rss.xml", rssHandler)
    log.Printf("listening on 0.0.0.0:%s", port)
    http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}
