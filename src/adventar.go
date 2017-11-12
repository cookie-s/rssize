package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/pkg/errors"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

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

	client := urlfetch.Client(ctx)
	resp, err := client.Get(url)
	if err != nil {
		return Data{}, errors.Wrapf(err, "client.Get fail %s", calid)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
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

func adventarHandler(c echo.Context) error {
	calid_s := c.Param("calid")
	/*
		calid, err := strconv.Atoi(calid_s)
		if err != nil {
			return c.String(http.StatusNotFound, "Not Found")
		}
	*/

	req := c.Request().(*standard.Request).Request
	ctx := appengine.NewContext(req)
	data, err := getData(ctx, calid_s)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Error")
	}

	return c.String(http.StatusOK, data.PropData.Entries[0].Title)
}
