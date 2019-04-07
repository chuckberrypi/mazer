package main

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"log"
	"os"
)

//direction values should only be up, down, left, or right
type direction int

const (
	up = iota
	down
	right
	left
)

var throttle chan struct{}
var solution chan *image.RGBA
var deadEnd chan *image.RGBA
var node chan *image.RGBA

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
	Image     *image.RGBA
	History   []position
	paths     []direction
	LineColor color.RGBA
	BkgColor  color.RGBA
}

func main() {
	//throttle puts a cap on the number of files that the program
	//opens at any time. may not be necessary with new channel
	//design

	throttle = make(chan struct{}, 30)
	solution = make(chan *image.RGBA)
	deadEnd = make(chan *image.RGBA, 100)
	node = make(chan *image.RGBA, 100)

	file := getMaze()
	defer file.Close()

	mazeGif, err := gif.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	initialMazeImage := trimMaze(mazeGif)

	initialMaze := firstMazePath(initialMazeImage)

	saveImageAsGif(initialMazeImage, "./unsolved_maze.gif")
	tc := make(chan bool)

	go solveMaze(initialMaze, tc)

	images := make([]*image.RGBA, 0)

	var im *image.RGBA
	b := true
	for b {
		select {
		case im = <-node:
			images = append(images, im)
		case im = <-deadEnd:
			images = append(images, im)
		case im = <-solution:
			fmt.Println("Solution found.")
			nodeLen := len(node)
			for i := 0; i < nodeLen; i++ {
				images = append(images, <-node)
			}
			images = append(images, im)
			b = false
		case b = <-tc:
			if !b {
				log.Fatal("Program found no solution.")
				break
			}
		}
	}

	saveImageAsGif(images[len(images)-1], "solved.gif")
	solvedMotion := makeGIF(images)
	gifFile, err := os.Create("motionSolve.gif")
	if err != nil {
		log.Fatal(err)
	}
	defer gifFile.Close()
	gif.EncodeAll(gifFile, &solvedMotion)

}

func makeGIF(images []*image.RGBA) gif.GIF {
	gifImages := make([]*image.Paletted, 0)
	delays := make([]int, 0)
	fmt.Printf("There are %v images to convert to a GIF.\n", len(images))
	for n, i := range images {
		fmt.Printf("Appending RGBA no. %v to slice of *image.Paletted\n", n)
		gifImages = append(gifImages, rgbaToPalette(i))
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
	throttle <- struct{}{}
	output, err := os.Create(n)
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	gif.Encode(output, i, nil)
	<-throttle
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

func drawPath(m mazePath) *image.RGBA {
	retImage := copyRGBA(m.Image)

	historyLen := len(m.History)
	for i := 1; i < historyLen; i++ {
		currentPosition := m.History[i]
		lastPosition := m.History[i-1]
		dir := directionTraveled(currentPosition, lastPosition)
		draw(lastPosition, retImage, dir)
	}

	return retImage
}

func draw(start position, i *image.RGBA, dir direction) {
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

func solveMaze(m mazePath, tChan chan bool) {

	if len(m.paths) <= 0 {
		deadEnd <- drawPath(m)
		tChan <- false
	} else if !exitFound(m) {
		//	node <- drawPath(m)
		tc := make(chan bool)

		pathLen := len(m.paths)

		for i := 0; i < pathLen; i++ {
			dir := m.paths[i]
			nextMaze := nextMazePath(m, dir)
			go solveMaze(nextMaze, tc)
		}

		tflag := true

		for i := 0; i < pathLen; i++ {
			b := <-tc
			if b {
				tChan <- true
				tflag = false
				break
			}
		}
		if tflag {
			tChan <- false
		}
	} else {
		solution <- drawPath(m)
		tChan <- true
	}
}

func firstMazePath(i *image.RGBA) mazePath {
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
