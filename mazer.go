package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
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
	throttle = make(chan struct{}, 30)

	file := getMaze()
	defer file.Close()

	mazeGif, err := gif.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	initialMazeImage := trimMaze(mazeGif)

	fmt.Printf("Positions: \n")

	fmt.Printf("initialMazeImage.Bounds(): %v\n", initialMazeImage.Bounds())

	initialMaze := firstMazePath(initialMazeImage)

	saveImageAsGif(initialMazeImage, "./unsolved_maze.gif")

	c := make(chan mazeError)
	deadChannel := deadEndNames()

	solveMaze(initialMaze, c, deadChannel)

	solution := <-c

	if solution.E != nil {
		log.Fatal("There does not seem to be a solution :(")
	} else {
		fmt.Println("Hunky Dory")
	}

	finalImage := drawPath(solution.M)

	saveImageAsGif(finalImage, "./solved_maze.gif")

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

func solveMaze(m mazePath, mc chan mazeError, deadCh chan string) {
	var solvedMaze mazePath
	if exitFound(m) {
		fmt.Printf("Exit was found...\n")
		mc <- mazeError{M: m, E: nil}
	} else if len(m.paths) <= 0 {
		deadEnd(m, deadCh)
		mc <- mazeError{M: solvedMaze, E: errors.New("No paths left")}
	} else {
		c := make(chan mazeError)
		pathLen := len(m.paths)

		for i := 0; i < pathLen; i++ {
			dir := m.paths[i]
			nextMaze := nextMazePath(m, dir)
			go solveMaze(nextMaze, c, deadCh)
		}

		for i := 0; i < pathLen; i++ {
			result := <-c
			if result.E == nil {
				mc <- result
			}
		}
		mc <- mazeError{M: solvedMaze, E: errors.New("No joy this way")}
	}
}

func deadEnd(m mazePath, dc chan string) {
	//TODO function that saves the image of an unsolved maze to make an animated gif
	name := "deadends/" + <-dc

	pathImage := drawPath(m)

	saveImageAsGif(pathImage, name)

	fmt.Printf("dead end func: %v\n", name)
	printMazeHistoryandPaths(m)
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
