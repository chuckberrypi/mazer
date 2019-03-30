package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/anaskhan96/soup"
)

func getMaze() *os.File {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Rows: ")
	entry, _ := reader.ReadString('\n')

	height, err := strconv.Atoi(strings.TrimRight(entry, "\n"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Columns: ")
	entry, _ = reader.ReadString('\n')

	width, err := strconv.Atoi(strings.TrimRight(entry, "\n"))
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{}

	headerMap := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		"Accept-Encoding":           "gzip, deflate",
		"Accept-Language":           "en-US,en;q=0.9",
		"Cache-Control":             "max-age=0",
		"Connection":                "keep-alive",
		"Content-Length":            "45",
		"Content-Type":              "application/x-www-form-urlencoded",
		"Host":                      "www.delorie.com",
		"Origin":                    "http://www.delorie.com",
		"Referer":                   "http://www.delorie.com/game-room/mazes/genmaze.cgi",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/72.0.3626.109 Safari/537.36",
	}

	form := url.Values{}
	form.Add("cols", strconv.Itoa(width))
	form.Add("rows", strconv.Itoa(height))

	form.Add("type", "gif")

	req, err := http.NewRequest("POST", "http://www.delorie.com/game-room/mazes/genmaze.cgi", strings.NewReader(form.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	for key, val := range headerMap {
		req.Header.Add(key, val)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	gzipReader, err4 := gzip.NewReader(resp.Body)
	if err4 != nil {
		log.Fatal(err)
	}

	bodyBytes, err2 := ioutil.ReadAll(gzipReader)
	if err2 != nil {
		log.Fatal(err)
	}

	bodyString := string(bodyBytes)

	bodySoup := soup.HTMLParse(bodyString)

	baseURL := strings.TrimRight(bodySoup.Find("base").Attrs()["href"], "genmaze.cgi")
	fmt.Printf("Base url: %s\n", baseURL)

	mapURL := baseURL + bodySoup.Find("img").Attrs()["src"]

	mazeResp, err := http.Get(mapURL)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Create("./maze.gif")
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	_, err = io.Copy(file, mazeResp.Body)
	if err != nil {
		log.Fatal(err)
	}

	file, err = os.Open("./maze.gif")
	return file

}

func trimMaze(i image.Image) *image.RGBA {

	imageAlphaPoint := getFirstLinePoint(i)
	imageOmegaPoint := getLastLinePoint(i)

	// iWidth := i.Bounds().Max.X - i.Bounds().Min.X
	// iHeight := i.Bounds().Max.Y - i.Bounds().Min.Y

	// xShift := (imageAlphaPoint.X - i.Bounds().Min.X) + (i.Bounds().Max.X - imageOmegaPoint.X)
	// yShift := (imageAlphaPoint.Y - i.Bounds().Min.Y) + (i.Bounds().Max.Y - imageOmegaPoint.Y)

	retImage := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{X: (imageOmegaPoint.X - imageAlphaPoint.X) + 1, Y: (imageOmegaPoint.Y - imageAlphaPoint.Y) + 1}})
	for x := retImage.Bounds().Min.X; x <= retImage.Bounds().Max.X+1; x++ {
		for y := retImage.Bounds().Min.Y; y <= retImage.Bounds().Max.Y+1; y++ {
			retImage.Set(x, y, i.At(x+(imageAlphaPoint.X-i.Bounds().Min.X), y+(imageAlphaPoint.Y-i.Bounds().Min.Y)))
		}
	}
	return retImage
}

func getFirstLinePoint(i image.Image) image.Point {
	clr := i.At(i.Bounds().Min.X, i.Bounds().Min.Y)
	for x := i.Bounds().Min.X; x < i.Bounds().Max.X; x++ {
		for y := i.Bounds().Min.Y; y < i.Bounds().Max.Y; y++ {
			if i.At(x, y) != clr {
				return image.Point{X: x, Y: y}
			}
		}
	}
	return image.Point{0, 0}
}

func getLastLinePoint(i image.Image) image.Point {
	clr := i.At(i.Bounds().Min.X, i.Bounds().Min.Y)
	for x := i.Bounds().Max.X - 1; x > i.Bounds().Min.X; x-- {
		for y := i.Bounds().Max.Y - 1; y > i.Bounds().Min.Y; y-- {
			if i.At(x, y) != clr {
				return image.Point{X: x, Y: y}
			}
		}
	}
	return image.Point{0, 0}
}
