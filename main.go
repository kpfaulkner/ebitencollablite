package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/kpfaulkner/collablite/client"
	"github.com/kpfaulkner/collablite/cmd/common"
)

type box struct {
	colour color.RGBA

	// top left point
	image.Point
}

type Game struct {
	boxSize       int
	boxes         map[image.Point]box
	screenwidth   int
	screenheight  int
	widthInBoxes  int
	heightInBoxes int

	cli *client.Client
	ctx context.Context

	sending       bool
	boxUpdateLock sync.Mutex

	// indicates update/draw should do its thing
	readyToRun bool

	// do we have full/orig doc loaded?
	fullDocLoaded bool

	// temporary buffer used to store updates while the full document is being loaded.
	buffer []box

	keepRed   bool
	keepBlue  bool
	keepGreen bool

	keepValue int

	updateSpeedCount int

	callbackCount int

	rps int
}

func NewGame(boxSize int, widthInBoxes int, heightInBoxes int) *Game {
	g := Game{}
	g.boxes = make(map[image.Point]box)
	g.boxSize = boxSize
	g.screenwidth = widthInBoxes * boxSize
	g.screenheight = heightInBoxes * boxSize
	g.fullDocLoaded = false
	g.readyToRun = false
	g.readyToRun = false
	g.ctx = context.Background()
	g.cli = client.NewClient("localhost:50051")
	g.cli.RegisterCallback(g.cb)
	g.widthInBoxes = widthInBoxes
	g.heightInBoxes = heightInBoxes

	g.buffer = make([]box, 1000)

	err := g.cli.Connect(g.ctx)
	if err != nil {
		log.Fatalf("Unable to connect to server: %v", err)
	}

	go g.cli.Listen(g.ctx)

	err = g.cli.RegisterToObject(g.ctx, "graphical")
	if err != nil {
		log.Fatalf("Unable to register object server: %v", err)
	}

	return &g
}

func (g *Game) LoadOriginalObject(objectID string) error {
	// registration and listening done... now load the full doc.
	origChanges, err := g.cli.GetObject("graphical")
	if err != nil {
		log.Fatalf("unable to get original object: %v", err)
	}

	for _, c := range origChanges {

		// yes this is duped code.. just trying to see if the idea works. If so, will refactor to be common. FIXME(kpfaulkner)
		if c.PropertyID != "" {
			sp := strings.Split(c.PropertyID, "-")
			x, _ := strconv.Atoi(sp[0])
			y, _ := strconv.Atoi(sp[1])
			b := box{colour: color.RGBA{c.Data[0], c.Data[1], c.Data[2], c.Data[3]}, Point: image.Point{x, y}}
			g.boxes[image.Point{x, y}] = b
		}
	}

	// now take anything in the buffer and add to boxes
	for _, b := range g.buffer {
		g.boxes[b.Point] = b
	}

	// clear the buffer
	g.buffer = []box{}

	g.fullDocLoaded = true
	g.readyToRun = true
	return nil
}

func (g *Game) cb(c *client.ChangeConfirmation) error {
	if c.PropertyID != "" {

		g.callbackCount++
		sp := strings.Split(c.PropertyID, "-")
		x, _ := strconv.Atoi(sp[0])
		y, _ := strconv.Atoi(sp[1])

		b := box{colour: color.RGBA{c.Data[0], c.Data[1], c.Data[2], c.Data[3]}, Point: image.Point{x, y}}

		// if full/orig doc not loaded, then do NOT put these updates into g.boxes but put them in the buffer.
		if !g.fullDocLoaded {
			g.buffer = append(g.buffer, b)
			return nil
		}

		// otherwise... update the box.
		g.boxUpdateLock.Lock()
		g.boxes[image.Point{x, y}] = b
		g.boxUpdateLock.Unlock()
	}

	return nil
}

func (g *Game) generateBoxes() {
	w := g.screenwidth / g.boxSize
	h := g.screenheight / g.boxSize
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			g.boxes[image.Point{x, y}] = box{colour: color.RGBA{0, 0, 0, 255}, Point: image.Point{x, y}}
		}
	}
}

func (g *Game) Update() error {

	//t := time.Now()
	// generate own changes... send them off... and also get confirmation.
	// do 10 changes.

	g.updateSpeedCount++

	modder := ebiten.TPS() / g.rps
	if g.updateSpeedCount >= ebiten.TPS() {
		g.updateSpeedCount = 0
	}

	if g.updateSpeedCount%modder != 0 {
		return nil
	}

	var rr byte
	var bb byte
	var gg byte
	if g.readyToRun {
		if g.sending {
			for i := 0; i < 1; i++ {
				x := rand.Intn(g.widthInBoxes)
				y := rand.Intn(g.heightInBoxes)
				prop := fmt.Sprintf("%d-%d", x, y)

				rr = 0
				gg = 0
				bb = 0

				if g.keepRed {
					rr = byte(rand.Intn(255))
				}

				if g.keepBlue {
					bb = byte(rand.Intn(255))
				}
				if g.keepGreen {
					gg = byte(rand.Intn(255))
				}

				g.boxUpdateLock.Lock()
				g.boxes[image.Point{x, y}] = box{colour: color.RGBA{rr, gg, bb, 255}, Point: image.Point{x, y}}
				g.boxUpdateLock.Unlock()

				change := client.OutgoingChange{ObjectID: "graphical", PropertyID: prop, Data: []byte{rr, gg, bb, 255}}
				err := g.cli.SendChange(&change)
				if err != nil {
					log.Fatalf("Cannot send %v", change)
				}
			}
		}

	}
	//fmt.Printf("update took %d ms\n", time.Since(t).Milliseconds())
	return nil
}

// generateHashOfBoxes is just to generate something unique so we can compare results on multiple
// instances (as opposed to looking at colours)  :)
func (g *Game) generateHashOfBoxes() string {
	// just add it up the red values of all the pixels... good enough

	red := 0
	g.boxUpdateLock.Lock()
	for _, v := range g.boxes {
		red += int(v.colour.R)
	}
	g.boxUpdateLock.Unlock()
	return fmt.Sprintf("%d", red)
}

func (g *Game) Draw(screen *ebiten.Image) {

	//t := time.Now()

	if g.readyToRun {
		g.boxUpdateLock.Lock()
		for _, box := range g.boxes {
			ebitenutil.DrawRect(screen, float64(box.X*g.boxSize), float64(box.Y*g.boxSize), float64(g.boxSize), float64(g.boxSize), box.colour)
		}
		g.boxUpdateLock.Unlock()
		//fmt.Printf("draw took %d ms\n", time.Since(t).Milliseconds())

	}
	ebitenutil.DebugPrint(screen, fmt.Sprintf("CONFLICTS : %d", g.cli.GetConflictsCount()))
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("HASH : %s", g.generateHashOfBoxes()), 0, 20)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("CB : %d", g.callbackCount), 0, 30)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("CHANGES : %d", g.cli.GetChangeCount()), 0, 40)

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return g.screenwidth, g.screenheight
}

func main() {

	logLevel := flag.String("loglevel", "info", "Log Level: debug, info, warn, error")
	send := flag.Bool("send", false, "send updates")
	red := flag.Bool("red", false, "keep red static")
	green := flag.Bool("green", false, "keep green static")
	blue := flag.Bool("blue", false, "keep blue static")
	rps := flag.Int("rps", 10, "requests/updates per second")
	flag.Parse()

	common.SetLogLevel(*logLevel)

	rand.Seed(time.Now().Unix())
	g := NewGame(50, 10, 10)
	g.sending = *send
	g.generateBoxes()
	g.keepBlue = *blue
	g.keepGreen = *green
	g.keepRed = *red
	g.rps = *rps

	err := g.LoadOriginalObject("graphical")
	if err != nil {
		log.Fatalf("unable to load original object: %v", err)
	}

	// hack
	time.Sleep(2 * time.Second)

	var title string
	if g.keepRed {
		title = "red"
	}
	if g.keepGreen {
		title = "green"
	}

	if g.keepBlue {
		title = "blue"
	}

	ebiten.SetWindowSize(500, 500)
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}