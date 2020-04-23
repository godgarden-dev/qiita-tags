package main

import (
	"context"
	"encoding/json"
	"github.com/gocarina/gocsv"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"
)

const (
	Endpoint       string = "https://qiita.com/api/v2"
	DefaultPerPage int    = 100
)

type Client struct {
	URL        *url.URL
	HTTPClient *http.Client
}

type Tag struct {
	FollowersCount int    `json:"followers_count" csv:"followers_count"`
	IconURL        string `json:"icon_url" csv:"icon_url"`
	ID             string `json:"id" csv:"id"`
	ItemsCount     int    `json:"items_count" csv:"items_count"`
}

func main() {

	log.Println("INFO:START")

	client, err := NewClient(Endpoint)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	tags, err := client.listTags(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := output(tags); err != nil {
		log.Fatal(err)
	}

	log.Println("INFO:END")
}

func NewClient(urlStr string) (*Client, error) {
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse url: %s", urlStr)
	}
	return &Client{URL: parsedURL, HTTPClient: http.DefaultClient}, nil
}

func (c *Client) newRequest(ctx context.Context, method, spath string, body io.Reader) (*http.Request, error) {
	u := *c.URL
	u.Path = path.Join(c.URL.Path, spath)

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer 0baac375a105d668bfd515b8b6c98ca585a5ec44")

	return req, nil
}

func (c *Client) listTags(ctx context.Context) ([]Tag, error) {
	page := 1

	req, err := c.newRequest(ctx, "GET", "/tags", nil)
	if err != nil {
		return nil, err
	}
	q := url.Values{
		"page":     []string{strconv.Itoa(page)},
		"per_page": []string{strconv.Itoa(DefaultPerPage)},
		"sort":     []string{"count"},
	}
	req.URL.RawQuery = q.Encode()

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		log.Printf("status:%d", res.StatusCode)
		return nil, nil
	}

	var tagList []Tag
	if err := decodeBody(res, &tagList); err != nil {
		return nil, err
	}

	totalCount, _ := strconv.Atoi(res.Header.Get("Total-Count"))
	maxPage := int(totalCount / DefaultPerPage)

	for currentPage := page + 1; currentPage <= maxPage; currentPage++ {
		req, err := c.newRequest(ctx, "GET", "/tags", nil)
		if err != nil {
			return nil, err
		}

		q.Set("page", strconv.Itoa(currentPage+1))
		req.URL.RawQuery = q.Encode()

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != 200 {
			log.Printf("break!! status:%d", res.StatusCode)
			break
		}

		var tags []Tag
		if err := decodeBody(res, &tags); err != nil {
			return nil, err
		}

		tagList = append(tagList, tags...)
	}

	return tagList, nil
}

func decodeBody(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(out)
}

func output(tags []Tag) error {
	file, err := os.OpenFile("/tmp/qiita_tags.csv", os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	if err := gocsv.MarshalFile(&tags, file); err != nil {
		log.Fatal(err)
	}
	return nil
}
