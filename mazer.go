package main

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
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

//direction values should only be up, down, left, or right
type direction int

const (
	up = iota
	down
	right
	left
)

type position struct {
	X, Y int
}

type mazePath struct {
	Maze      *image.RGBA
	History   []position
	paths     []direction
	LineColor color.RGBA
	BkgColor  color.RGBA
}

func main() {

	file := getMaze()
	defer file.Close()

	mazeGif, err := gif.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	initialMazeImage := trimMaze(mazeGif)

	output, err := os.Create("./unsolved_maze.gif")
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	initialMazeImage.Set(5, 5, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	gif.Encode(output, initialMazeImage, nil)

	initialMaze := firstMazePath(initialMazeImage)

	solvedMaze, err := solveMaze(initialMaze)

}

func solveMaze(m mazePath) (mazePath, error) {
	var solvedMaze mazePath
	if exitFound(m) {
		return m, nil
	} else if len(m.paths) <= 0 {
		return solvedMaze, errors.New("No paths left")
	} else {
		for len(m.paths) > 0 {
			dir := m.paths[len(m.paths)-1]
			m.paths = m.paths[:len(m.paths)-1]
			nextMazePath := nextMazePath(m, dir)
			solvedMaze, err := solveMaze(nextMazePath)
			if err == nil {
				break
			}
		}
		return solvedMaze, err
	}
}

func firstMazePath(i *image.RGBA) mazePath {
	paths := make([]direction, 0)
	history := make([]position, 0)
	history = append(history, position{X: 5, Y: 5})
	maze := i
	bkgColor, lineColor := getBkgColorLineColor(i)
	mp := mazePath{
		Maze:      maze,
		History:   history,
		paths:     paths, //need to calculate actual opptions after
		LineColor: lineColor,
		BkgColor:  bkgColor,
	}
	mp.paths = append(mp.paths, options(mp)...)
	return mp
}

func nextMazePath(m mazePath, d direction) mazePath {
	paths := make([]direction, 0)
	history := make([]position, 0)
	nextPos := nextPosition(m, d)
	history = append(history, m.History...)
	history = append(history, nextPos)
	maze := copyRGBA(m.Maze)
	mp := mazePath{
		Maze:      maze,
		History:   history,
		paths:     paths,
		LineColor: m.LineColor,
		BkgColor:  m.BkgColor,
	}
	mp.paths = append(m.paths, options(mp)...)
	return mp
}

func exitFound(m mazePath) bool {
	currentPosition := m.History[len(m.History)-1]
	if directionSliceContains(m.paths, left) {
		if m.Maze.Bounds().Max.X == currentPosition.X+5 {
			return true
		}
	}
	return false
}

func directionSliceContains(ints []direction, n direction) bool {
	for _, num := range ints {
		if num == n {
			return true
		}
	}
	return false
}

func nextPosition(m mazePath, d direction) position {
	var pos position
	currentPosition := m.History[len(m.History)-1]
	pos.X = currentPosition.X
	pos.Y = currentPosition.Y
	switch d {
	case up:
		pos.Y = currentPosition.Y - 10
	case down:
		pos.Y = currentPosition.Y + 10
	case right:
		pos.X = currentPosition.X + 10
	default:
		pos.X = currentPosition.X - 10
	}
	return pos
}

func options(m mazePath) []direction {

	directions := make([]direction, 0)
	currentPosition := m.History[len(m.History)-1]

	if m.Maze.At(currentPosition.X+5, currentPosition.Y) != m.LineColor {
		directions = append(directions, right)
		fmt.Println("appending right")
	}

	if m.Maze.At(currentPosition.X-5, currentPosition.Y) != m.LineColor && len(m.History) != 1 {
		directions = append(directions, left)
		fmt.Println("appending left")
	}

	if m.Maze.At(currentPosition.X, currentPosition.Y-5) != m.LineColor {
		directions = append(directions, up)
		fmt.Println("appending up")
	}

	if m.Maze.At(currentPosition.X, currentPosition.Y+5) != m.LineColor {
		directions = append(directions, down)
		fmt.Println("appending down")
	}

	incoming := incomingDirection(m)
	var disallowed direction

	switch incoming {
	case up:
		disallowed = down
	case down:
		disallowed = up
	case left:
		disallowed = right
	default:
		disallowed = left
	}

	directions = filterDirections(directions, func(n direction) bool {
		if n == disallowed {
			return false
		}
		return true
	})

	return directions
}

func filterDirections(ds []direction, f func(direction) bool) []direction {
	newDirections := make([]direction, 0)
	for _, d := range ds {
		if f(d) {
			newDirections = append(newDirections, d)
		}
	}
	return newDirections
}

func incomingDirection(m mazePath) direction {
	if len(m.History) == 1 {
		return right
	} else {
		currentPosition := m.History[len(m.History)-1]
		lastPosition := m.History[len(m.History)-2]
		return directionTraveled(lastPosition, currentPosition)
	}
}

func inBounds(p position, r image.Rectangle) bool {
	if r.Min.X <= p.X && r.Max.X >= p.X && r.Min.Y <= p.Y && r.Max.Y >= p.Y {
		return true
	}
	return false
}

//direction takes two points and tells you whether the cursor moved
//up, down, left, or right to get from the first to the second position
func directionTraveled(first, second position) direction {
	if second.X == first.X {
		if second.Y > first.Y {
			return down
		} else {
			return up
		}
	} else {
		if second.X > first.X {
			return right
		} else {
			return left
		}
	}
}

func lastPosition(m mazePath) position {
	pos := m.History[len(m.History)-1]
	return pos
}

func copyRGBA(i *image.RGBA) *image.RGBA {
	retImage := image.NewRGBA(i.Rect)
	copy(retImage.Pix, i.Pix)
	retImage.Stride = i.Stride
	return retImage
}

func getCellWidth(i *image.RGBA) int {
	_, lineColor := getBkgColorLineColor(i)
	fmt.Println(i.At(i.Bounds().Min.X, i.Bounds().Min.Y+100) == lineColor)
	counter := 0
	for y := i.Bounds().Min.Y; y < i.Bounds().Max.Y; y++ {
		if i.At(0, y) != lineColor {
			counter++
		}
	}

	fmt.Printf("Max y: %v\n", i.Bounds().Max.Y)

	return counter
}

//RGBA32toRGBA8 looks better with a capital letter
func RGBA32toRGBA8(r, g, b, a uint32) color.RGBA {
	col := color.RGBA{R: uint8(r / 0x101), G: uint8(g / 0x101), B: uint8(b / 0x101), A: uint8(a / 0x101)}
	return col
}

func getBkgColorLineColor(i *image.RGBA) (bkgColor, lineColor color.RGBA) {
	lineColor = RGBA32toRGBA8(i.At(i.Bounds().Min.X, i.Bounds().Min.Y).RGBA())
	currentColor := lineColor

	for y := i.Bounds().Min.Y; y < i.Bounds().Max.Y && currentColor == lineColor; y++ {
		for x := i.Bounds().Min.X; x < i.Bounds().Max.X && currentColor == lineColor; x++ {
			if i.At(x, y) != currentColor {
				currentColor = RGBA32toRGBA8(i.At(x, y).RGBA())
				bkgColor = currentColor
			}
		}

	}
	return
}

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
