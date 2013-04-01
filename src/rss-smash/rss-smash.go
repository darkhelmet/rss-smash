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
        "http://comicsrss.herokuapp.com/thedoghousediaries",
        "http://comicsrss.herokuapp.com/cyanide",
        "http://www.questionablecontent.net/QCRSS.xml",
        "http://twitterthecomic.tumblr.com/rss",
        "http://www.xkcd.com/rss.xml",
        "http://www.rsspect.com/rss/qwantz.xml",
    }
    Funcs = T.FuncMap{
        "Safe":  func(s string) T.HTML { return T.HTML(s) },
        "CDATA": func(s string) T.HTML { return T.HTML(fmt.Sprintf("<![CDATA[%s]]>", s)) },
    }
    Feed = T.Must(T.New("rss").Funcs(Funcs).Parse(`{{"<?xml version=\"1.0\" encoding=\"UTF-8\"?>" | Safe}}
<rss version="2.0">
    <channel>
        <title>RSS! SMASH!</title>
        <link>http://rss-smash.herokuapp.com/rss.xml</link>
        <description>An RSS mashup of all my comics</description>
        {{range .}}
        <item>
            <title>{{.Title}}</title>
            <link>{{.Link}}</link>
            <description>{{.Description | CDATA}}</description>
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

func fetchFeedItems(url string, items chan *rss.Item) {
    channelHandler := func(f *rss.Feed, newchannels []*rss.Channel) {
        for _, channel := range newchannels {
            log.Printf("got new channel %s with %d items", channel.Title, len(channel.Items))
            for _, item := range channel.Items {
                items <- item
            }
            close(items)
        }
    }

    go func() {
        feed := rss.New(5, true, channelHandler, nil)
        err := feed.Fetch(url, nil)
        if err != nil {
            log.Printf("failed fetching %s: %s", url, err)
            close(items)
        }
    }()
}

func parseTime(s string) (time.Time, error) {
    t, err := time.Parse(time.RFC1123Z, s)
    if err != nil {
        return time.Parse(time.RFC1123, s)
    }
    return t, err
}

func fetchAllFeedItems(urls []string) (items SortedItems) {
    var ichans []chan *rss.Item

    for _, url := range Feeds {
        ichan := make(chan *rss.Item)
        fetchFeedItems(url, ichan)
        ichans = append(ichans, ichan)
    }

    for _, ichan := range ichans {
        for item := range ichan {
            pubdate, err := parseTime(item.PubDate)
            if err == nil {
                items = append(items, &Item{
                    Title:       item.Title,
                    Link:        item.Links[0].Href,
                    Description: item.Description,
                    Guid:        item.Guid,
                    PubDate:     pubdate.UTC(),
                })
            }
        }
    }

    log.Println("done retrieving items")

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
