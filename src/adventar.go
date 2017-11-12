package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/mjibson/goon"
	"github.com/pkg/errors"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"
)

type AdventarLastUpdated struct {
	EntryID int64     `datastore:"-" goon:"id"`
	Updated time.Time `datastore:"updated"`
}

type Entry struct {
	ID      int    `json:"id"`
	Date    string `json:"date"`
	Image   string `json:"image"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Comment string `json:"comment"`
}

type PropData struct {
	Calendar struct {
		ID   int `json:"id"`
		Year int `json:"year"`
	} `json:"calendar"`
	Entries []Entry `json:"entries"`
}

type Data struct {
	PageTitle string
	Desc      string
	PageURL   string
	PropData  PropData
}

func getTitle(doc *goquery.Document) string {
	sel := `title`
	tit := doc.Find(sel).First().Text()
	return tit
}

func getDesc(doc *goquery.Document) string {
	sel := `meta[name="description"]`
	desc := doc.Find(sel).First().Text()
	return desc
}

func getPropData(doc *goquery.Document) (PropData, error) {
	sel := `div[data-react-class="CalendarContainer"]`
	props, exists := doc.Find(sel).First().Attr("data-react-props")
	if exists == false {
		return PropData{}, errors.New("prop not found")
	}

	data := PropData{}
	if err := json.Unmarshal([]byte(props), &data); err != nil {
		return PropData{}, errors.Wrap(err, "unmarshal prop fail")
	}
	return data, nil
}

func getData(ctx context.Context, calid string) (Data, error) {
	url := "https://adventar.org/calendars/" + calid

	var reader io.Reader

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("rssize"+url)))
	cache, err := memcache.Get(ctx, hash)
	if err == nil {
		reader = bytes.NewReader(cache.Value)
	} else if err == memcache.ErrCacheMiss {
		client := urlfetch.Client(ctx)
		resp, err := client.Get(url)
		if err != nil {
			return Data{}, errors.Wrapf(err, "client.Get fail %s", calid)
		}
		defer resp.Body.Close()
		val, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return Data{}, errors.Wrapf(err, "ReadAll fail %s", calid)
		}
		cache := memcache.Item{
			Key:        hash,
			Value:      val,
			Expiration: time.Minute * 30,
		}
		_ = memcache.Add(ctx, &cache)

		reader = bytes.NewReader(val)
	} else {
		return Data{}, errors.Wrapf(err, "Get memcache fail %s", calid)
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return Data{}, errors.Wrapf(err, "goquery parse fail %s", calid)
	}

	title := getTitle(doc)
	desc := getDesc(doc)
	pdata, err := getPropData(doc)
	if err != nil {
		return Data{}, err
	}

	return Data{PageTitle: title, Desc: desc, PageURL: url, PropData: pdata}, nil
}

func AdventarHandler(c echo.Context) error {
	calid_s := c.Param("calid")
	_, err := strconv.Atoi(calid_s)
	if err != nil {
		return c.String(http.StatusNotFound, "Not Found")
	}

	req := c.Request().(*standard.Request).Request
	ctx := appengine.NewContext(req)
	data, err := getData(ctx, calid_s)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Error")
	}

	feed := &feeds.Feed{
		Title:       data.PageTitle,
		Link:        &feeds.Link{Href: data.PageURL},
		Description: data.Desc,
		Created:     time.Now(),
	}

	g := goon.NewGoon(req)
	entries := data.PropData.Entries
	for _, entry := range entries {
		if entry.URL == "" {
			continue
		}

		updated := time.Now()

		q := AdventarLastUpdated{
			EntryID: int64(entry.ID),
		}
		err := g.Get(&q)
		if err == nil {
			updated = q.Updated
		} else if err == datastore.ErrNoSuchEntity {
			store := AdventarLastUpdated{
				EntryID: int64(entry.ID),
				Updated: time.Now(),
			}
			if _, err := g.Put(&store); err != nil {
				log.Println(err)
			}
		} else if err != nil {
			log.Println(err)
			continue // cont
		}

		item := &feeds.Item{
			Title:   entry.Title,
			Link:    &feeds.Link{Href: entry.URL},
			Created: updated,
		}
		feed.Items = append(feed.Items, item)
	}

	rss, err := feed.ToRss()
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Error")
	}

	return c.Blob(http.StatusOK, "text/xml", []byte(rss))
}
