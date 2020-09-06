package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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

var (
	red  = color.RGBA{R: 255, G: 0, B: 0, A: 0}
	blue = color.RGBA{R: 0, G: 0, B: 255, A: 0}
)

type mazeError struct {
	M mazePath
	E error
}

type position struct {
	X, Y int
}

type mazePath struct {
	Image     *image.Paletted
	History   []position
	paths     []direction
	LineColor color.RGBA
	BkgColor  color.RGBA
}

var solvedNameChannel chan string
var mazeGetThrottle chan struct{}
var timer chan struct{}
var mazePathChan chan mazePath

func main() {
	initializeGlobals()
	loops := getLoopNum()

	counts := make(map[int]int)
	longestSlice := 0
	shortestSlice := 99999999

	//vary the size of the maze to make sure that the website
	//doesn't just give the same maze multiple times.
	r := 60
	for i := 0; i < loops; i++ {

		go getMazePath(r)
		if r < 57 {
			r = 60
		} else {
			r--
		}

	}

	images := make([]*image.Paletted, 0)

	for i := 0; i < loops; i++ {
		initialMazePath := <-mazePathChan

		if solveMaze(initialMazePath, &images) {
			fmt.Printf("Maze No. %v was solved (%v dead ends)\n", i, len(images))
		} else {
			fmt.Println("Maze was not solved.")
		}

		sliceLen := len(images)

		counts[sliceLen] = counts[sliceLen] + 1
		if sliceLen > longestSlice {
			gifFile, err := os.Create("longSolvedInMotion.gif")
			if err != nil {
				log.Fatal(err)
			}
			solvedMotion := makeGIF(&images)
			gif.EncodeAll(gifFile, &solvedMotion)
			longestSlice = sliceLen
			gifFile.Close()
		}
		if sliceLen < shortestSlice {
			gifFile, err := os.Create("shortSolvedInMotion.gif")
			if err != nil {
				log.Fatal(err)
			}
			solvedMotion := makeGIF(&images)
			gif.EncodeAll(gifFile, &solvedMotion)
			shortestSlice = sliceLen
			gifFile.Close()
		}
		images = nil
	}

	for key, val := range counts {
		fmt.Printf("Len: %v; count: %v\n", key, val)
	}

	fmt.Printf("Shortest trek: %v wrong turns.\n", shortestSlice)
	fmt.Printf("Longest trek: %v wrong turns.\n", longestSlice)

}

func getLoopNum() int {
	fmt.Print("How many loops? ")
	stdreader := bufio.NewReader(os.Stdin)
	lines, err := stdreader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	loops, err := strconv.Atoi(strings.Trim(lines, " \n\t"))
	if err != nil {
		log.Fatal(err)
	}
	return loops
}

func initializeGlobals() {
	mazeGetThrottle = make(chan struct{}, 2)
	mazePathChan = make(chan mazePath)
	solvedNameChannel = serialNamer()
	timer = func() chan struct{} {
		c := make(chan struct{})
		go func() {
			for {
				time.Sleep(1000)
				c <- struct{}{}
			}
		}()
		return c
	}()
}

func getMazePath(size int) {
	mazeGetThrottle <- struct{}{}
	<-timer

	mazeGif := getMaze(size, size)

	initialMazeImage := rgbaToPalette(trimMaze(mazeGif))

	initialMaze := firstMazePath(initialMazeImage)

	mazePathChan <- initialMaze
	<-mazeGetThrottle
}

func serialNamer() chan string {
	num := 0
	retChan := make(chan string)
	go func(c chan string) {
		for {
			c <- fmt.Sprintf("motionSolved_%v.gif", num)
			num++
		}
	}(retChan)
	return retChan
}

func makeGIF(images *[]*image.Paletted) gif.GIF {
	gifImages := make([]*image.Paletted, 0)
	delays := make([]int, 0)
	for _, i := range *images {
		gifImages = append(gifImages, copyPaletted(i))
	}
	for i := 0; i < len(gifImages); i++ {
		delays = append(delays, 12)
	}
	delays[len(delays)-1] = 110
	theGif := gif.GIF{
		Image:     gifImages,
		Delay:     delays,
		LoopCount: 0,
	}
	return theGif
}

func rgbaToPalette(i *image.RGBA) *image.Paletted {

	retImage := image.NewPaletted(i.Bounds(), palette.Plan9)
	for y := retImage.Bounds().Min.Y; y < retImage.Bounds().Max.Y; y++ {
		for x := retImage.Bounds().Min.X; x < retImage.Bounds().Max.X; x++ {
			retImage.Set(x, y, retImage.ColorModel().Convert(i.At(x, y)))
		}
	}

	return retImage
}

func printMazeHistoryandPaths(m mazePath) {
	fmt.Printf("m.History: %v\n", m.History)
	fmt.Printf("m.paths: %v\n", dirsToStrings(m.paths))
}

func dirsToStrings(dir []direction) []string {
	str := make([]string, 0)
	for _, d := range dir {
		str = append(str, sPrintDirection(d))
	}
	return str
}

func saveImageAsGif(i *image.RGBA, n string) {

	output, err := os.Create(n)
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	gif.Encode(output, i, nil)

}

func deadEndNames() chan string {
	c := make(chan string)
	deadNum := 0
	go func(ch chan string) {
		for {
			deadNum++
			ch <- fmt.Sprintf("dead_end%v.gif", deadNum)
		}
	}(c)
	return c
}

func sPrintDirection(d direction) string {
	switch d {
	case up:
		return "up"
	case down:
		return "down"
	case right:
		return "right"
	default:
		return "left"
	}
}

func drawAllPoints(m mazePath, points []position) {
	for _, point := range points {
		if endPoint(m, point) {
			m.Image.Set(point.X, point.Y, blue)
		} else {
			m.Image.Set(point.X, point.Y, red)
		}
	}
}

func endPoint(m mazePath, p position) bool {
	if p.X+5 == m.Image.Bounds().Max.X-1 && m.Image.At(p.X+5, p.Y) != m.LineColor {
		return true
	}
	return false
}

func drawPath(m mazePath) *image.Paletted {
	retImage := copyPaletted(m.Image)

	historyLen := len(m.History)
	for i := 1; i < historyLen; i++ {
		currentPosition := m.History[i]
		lastPosition := m.History[i-1]
		dir := directionTraveled(currentPosition, lastPosition)
		draw(lastPosition, retImage, dir)
	}

	return retImage
}

func draw(start position, i *image.Paletted, dir direction) {
	pos := start
	i.Set(start.X, start.Y, color.RGBA{R: 255, G: 0, B: 0, A: 0})
	for g := 0; g < 10; g++ {
		pos = moveOne(pos, dir)
		i.Set(pos.X, pos.Y, color.RGBA{R: 255, G: 0, B: 0, A: 0})
	}
}

func moveOne(p position, d direction) position {
	pos := p
	switch d {
	case up:
		pos.Y--
	case down:
		pos.Y++
	case right:
		pos.X++
	default:
		pos.X--
	}
	return pos
}

func solveMaze(m mazePath, images *[]*image.Paletted) bool {

	if len(m.paths) <= 0 { //dead end
		*images = append(*images, drawPath(m))
		return false
	} else if !exitFound(m) { // not the solution and not a dead end.

		pathLen := len(m.paths)

		for i := 0; i < pathLen; i++ {
			dir := m.paths[i]
			nextMaze := nextMazePath(m, dir)
			if solveMaze(nextMaze, images) {
				return true
			}
		}
	} else { //the solution
		*images = append(*images, drawPath(m))
		return true
	}
	return false
}

func firstMazePath(i *image.Paletted) mazePath {
	paths := make([]direction, 0)
	history := make([]position, 0)
	history = append(history, position{X: 5, Y: 5})
	maze := i
	bkgColor, lineColor := getBkgColorLineColor(i)
	mp := mazePath{
		Image:     maze,
		History:   history,
		paths:     paths,
		LineColor: lineColor,
		BkgColor:  bkgColor,
	}
	mp.paths = filterDirections(append(mp.paths, options(mp, mp.History[len(mp.History)-1])...), func(d direction) bool {
		if d == left {
			return false
		}
		return true
	})
	return mp
}

func nextMazePath(m mazePath, d direction) mazePath {
	paths := make([]direction, 0)
	history := make([]position, 0)
	nextPos := nextPosition(m, d)
	history = append(history, m.History...)
	history = append(history, nextPos)
	mp := mazePath{
		Image:     m.Image,
		History:   history,
		paths:     paths,
		LineColor: m.LineColor,
		BkgColor:  m.BkgColor,
	}
	mp.paths = append(mp.paths, options(mp, mp.History[len(mp.History)-1])...)
	mp = cullPaths(mp)
	return mp
}

func cullPaths(m mazePath) mazePath {
	mp := m
	incoming := incomingDirection(m)
	mp.paths = filterDirections(m.paths, func(d direction) bool {
		if d == incoming {
			return false
		}
		return true
	})
	return mp
}

func exitFound(m mazePath) bool {
	currentPosition := m.History[len(m.History)-1]
	if directionSliceContains(m.paths, right) && m.Image.Bounds().Max.X-1 == currentPosition.X+5 {
		return true
	}
	return false
}

func directionSliceContains(dirs []direction, n direction) bool {
	for _, dir := range dirs {
		if dir == n {
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

func options(m mazePath, currentPosition position) []direction {

	directions := make([]direction, 0)

	if m.Image.At(currentPosition.X+5, currentPosition.Y) != m.LineColor {
		directions = append(directions, right)
	}

	if m.Image.At(currentPosition.X-5, currentPosition.Y) != m.LineColor {
		directions = append(directions, left)
	}

	if m.Image.At(currentPosition.X, currentPosition.Y-5) != m.LineColor {
		directions = append(directions, up)
	}

	if m.Image.At(currentPosition.X, currentPosition.Y+5) != m.LineColor {
		directions = append(directions, down)
	}

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
		return left
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
func directionTraveled(last, current position) direction {
	if last.X == current.X {
		if last.Y > current.Y {
			return down
		} else {
			return up
		}
	} else {
		if last.X > current.X {
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

func copyPaletted(i *image.Paletted) *image.Paletted {
	retImage := image.NewPaletted(i.Rect, palette.Plan9)
	copy(retImage.Pix, i.Pix)
	retImage.Stride = i.Stride
	return retImage
}

func getCellWidth(i *image.Paletted) int {
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

func getBkgColorLineColor(i *image.Paletted) (bkgColor, lineColor color.RGBA) {
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
func getMaze(rows, cols int) image.Image {

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
	form.Add("cols", strconv.Itoa(cols))
	form.Add("rows", strconv.Itoa(rows))

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

	mapURL := baseURL + bodySoup.Find("img").Attrs()["src"]

	mazeResp, err := http.Get(mapURL)
	if err != nil {
		log.Fatal(err)
	}

	image, err := gif.Decode(mazeResp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return image

}

func getRowsandCols() (rows, cols int) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Rows: ")
	entry, _ := reader.ReadString('\n')

	rows, err := strconv.Atoi(strings.Trim(entry, " \n\t"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Columns: ")
	entry, _ = reader.ReadString('\n')

	cols, err = strconv.Atoi(strings.Trim(entry, " \n\t"))
	if err != nil {
		log.Fatal(err)
	}

	return rows, cols
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
