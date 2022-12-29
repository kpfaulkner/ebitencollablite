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

	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text"
	log "github.com/sirupsen/logrus"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

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

type GameObject struct {
	boxes         map[image.Point]box
	boxUpdateLock sync.Mutex
}

func NewGameObject() *GameObject {
	g := GameObject{}
	g.boxes = make(map[image.Point]box)
	return &g
}

func (g *GameObject) GetBoxes() map[image.Point]box {
	newMap := make(map[image.Point]box, len(g.boxes))

	g.boxUpdateLock.Lock()
	defer g.boxUpdateLock.Unlock()
	for k, v := range g.boxes {
		newMap[k] = v
	}

	return newMap
}

type Game struct {
	boxSize       int
	object        *GameObject
	screenwidth   int
	screenheight  int
	widthInBoxes  int
	heightInBoxes int

	cli *client.Client
	ctx context.Context

	sending bool

	// indicates update/draw should do its thing
	readyToSend bool
	readyToDraw bool

	// do we have full/orig doc loaded?
	fullDocLoaded bool

	// temporary buffer used to store updates while the full document is being loaded.
	buffer []box

	keepRed   bool
	keepBlue  bool
	keepGreen bool

	keepValue int

	updateSpeedCount int

	callbackCount      int
	totalCallbackCount int
	sendCount          int

	rps int

	objectID string

	startTime      time.Time
	cbrps          float64
	sendsPerSec    float64
	nextUpdateTime time.Time
	updateDuration time.Duration
	mplusBigFont   font.Face
}

func NewGame(boxSize int, widthInBoxes int, heightInBoxes int, objectID string, host string) *Game {
	g := Game{}
	g.object = NewGameObject()
	g.boxSize = boxSize
	g.screenwidth = widthInBoxes * boxSize
	g.screenheight = heightInBoxes * boxSize
	g.ctx = context.Background()
	g.widthInBoxes = widthInBoxes
	g.heightInBoxes = heightInBoxes
	g.objectID = objectID
	g.buffer = make([]box, 1000)
	g.cli = client.NewClient(host)

	go g.connectAndRegister()

	return &g
}

func (g *Game) connectAndRegister() error {

	for {
		g.readyToSend = false
		g.cli.RegisterConverters(g.ConvertFromObject, g.ConvertToObject)
		err := g.cli.Connect(g.ctx)
		if err != nil {
			log.Fatalf("Unable to connect to server: %v", err)
		}

		err = g.cli.RegisterToObject(g.ctx, g.objectID)
		if err != nil {
			log.Fatalf("Unable to register object server: %v", err)
		}

		g.readyToSend = true
		log.Debugf("listen start")
		g.cli.Listen(g.ctx)
		log.Debugf("listen end")
		g.readyToSend = false
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (g *Game) LoadOriginalObject(objectID string) error {
	log.Debugf("LoadOriginalObject called")
	// registration and listening done... now load the full doc.
	origChanges, err := g.cli.GetObject(g.objectID)
	if err != nil {
		log.Fatalf("unable to get original object: %v", err)
	}

	for _, c := range origChanges {

		// yes this is duped code.. just trying to see if the idea works. If so, will refactor to be common. FIXME(kpfaulkner)
		if c.PropertyID != "" && c.PropertyID != g.objectID {
			sp := strings.Split(c.PropertyID, "-")
			x, _ := strconv.Atoi(sp[0])
			y, _ := strconv.Atoi(sp[1])
			b := box{colour: color.RGBA{c.Data[0], c.Data[1], c.Data[2], c.Data[3]}, Point: image.Point{x, y}}
			g.object.boxes[image.Point{x, y}] = b
		}
	}

	g.fullDocLoaded = true
	g.readyToSend = true
	g.readyToDraw = true
	log.Debugf("LoadOriginalObject end")
	return nil
}

func (g *Game) cb(change any) error {

	obj := change.(GameObject)
	g.object = &obj
	/*
		if obj.PropertyID != "" {

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
			g.object.boxes[image.Point{x, y}] = b
			g.boxUpdateLock.Unlock()
		} */

	return nil
}

func (g *Game) generateBoxes() {
	w := g.screenwidth / g.boxSize
	h := g.screenheight / g.boxSize
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			g.object.boxes[image.Point{x, y}] = box{colour: color.RGBA{0, 0, 0, 255}, Point: image.Point{x, y}}
		}
	}
}

func (g *Game) Update() error {

	//t := time.Now()
	// generate own changes... send them off... and also get confirmation.
	// do 10 changes.

	if time.Now().After(g.nextUpdateTime) {
		g.nextUpdateTime = time.Now().Add(g.updateDuration)
	} else {
		return nil
	}

	var rr byte
	var bb byte
	var gg byte
	if g.readyToSend {
		if g.sending {
			for i := 0; i < 1; i++ {
				x := rand.Intn(g.widthInBoxes)
				y := rand.Intn(g.heightInBoxes)
				prop := fmt.Sprintf("%d-%d", x, y)

				rr = 0
				gg = 0
				bb = 0

				if g.keepRed {
					rr = byte(rand.Intn(155) + 100)
				}

				if g.keepBlue {
					bb = byte(rand.Intn(155) + 100)
				}
				if g.keepGreen {
					gg = byte(rand.Intn(155) + 100)
				}

				g.object.boxUpdateLock.Lock()
				g.object.boxes[image.Point{x, y}] = box{colour: color.RGBA{rr, gg, bb, 255}, Point: image.Point{x, y}}
				g.object.boxUpdateLock.Unlock()

				change := client.OutgoingChange{ObjectID: g.objectID, PropertyID: prop, Data: []byte{rr, gg, bb, 255}}
				err := g.cli.SendObject(g.objectID, g.object)
				if err != nil {
					log.Errorf("Cannot send %v", change)
				}
				g.sendCount++
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
	g.object.boxUpdateLock.Lock()
	for _, v := range g.object.boxes {
		red += int(v.colour.R)
	}
	g.object.boxUpdateLock.Unlock()
	return fmt.Sprintf("%d", red)
}

func (g *Game) Draw(screen *ebiten.Image) {

	//t := time.Now()

	if g.readyToDraw {
		g.object.boxUpdateLock.Lock()

		for _, box := range g.object.boxes {
			ebitenutil.DrawRect(screen, float64(box.X*g.boxSize), float64(box.Y*g.boxSize), float64(g.boxSize), float64(g.boxSize), box.colour)
		}
		g.object.boxUpdateLock.Unlock()
		//fmt.Printf("draw took %d ms\n", time.Since(t).Milliseconds())

	}

	if time.Now().Sub(g.startTime) > time.Second {
		secsSinceStart := time.Since(g.startTime).Seconds()
		g.cbrps = float64(g.callbackCount) / secsSinceStart
		g.sendsPerSec = float64(g.sendCount) / secsSinceStart
		g.totalCallbackCount += g.callbackCount
		g.callbackCount = 0
		g.sendCount = 0
		g.startTime = time.Now()
	}

	text.Draw(screen, fmt.Sprintf("CONFLICTS : %d", g.cli.GetConflictsCount()), g.mplusBigFont, 0, 30, color.White)
	text.Draw(screen, fmt.Sprintf("HASH : %s", g.generateHashOfBoxes()), g.mplusBigFont, 0, 60, color.White)
	text.Draw(screen, fmt.Sprintf("TOT CB : %d", g.totalCallbackCount), g.mplusBigFont, 0, 90, color.White)
	text.Draw(screen, fmt.Sprintf("CB : %0.2f", g.cbrps), g.mplusBigFont, 0, 120, color.White)
	text.Draw(screen, fmt.Sprintf("SPS : %0.2f", g.sendsPerSec), g.mplusBigFont, 0, 150, color.White)
	text.Draw(screen, fmt.Sprintf("CHANGES : %d", g.cli.GetChangeCount()), g.mplusBigFont, 0, 180, color.White)

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return g.screenwidth, g.screenheight
}

func (g *Game) setupFont() {
	tt, err := opentype.Parse(fonts.MPlus1pRegular_ttf)
	if err != nil {
		log.Fatal(err)
	}

	const dpi = 72
	g.mplusBigFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Adjust the line height.
	g.mplusBigFont = text.FaceWithLineHeight(g.mplusBigFont, 54)

}
func main() {

	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred:", err)
		}
	}()

	host := flag.String("host", "10.0.0.108:50051", "host:port of server")
	objectID := flag.String("id", "graphical", "id of object")
	logLevel := flag.String("loglevel", "info", "Log Level: debug, info, warn, error")
	send := flag.Bool("send", false, "send updates")
	red := flag.Bool("red", false, "keep red static")
	green := flag.Bool("green", false, "keep green static")
	blue := flag.Bool("blue", false, "keep blue static")
	rps := flag.Int("rps", 10, "requests/updates per second")
	flag.Parse()

	common.SetLogLevel(*logLevel)

	rand.Seed(time.Now().Unix())
	g := NewGame(50, 10, 10, *objectID, *host)
	g.sending = *send
	g.generateBoxes()
	g.keepBlue = *blue
	g.keepGreen = *green
	g.keepRed = *red
	g.rps = *rps
	g.updateDuration = time.Second / time.Duration(g.rps)
	g.startTime = time.Now()

	g.setupFont()
	err := g.LoadOriginalObject(*objectID)
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
	ebiten.SetWindowTitle(fmt.Sprintf("%s  -  %s", title, *objectID))
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

// Converts from our internal object to what the client wants.
// Returns objectid, actual object and error.
// Keeping objectid separate due to not knowing if the client object has an objectid property.
// Only update the clientObject if the property has updated property field to true (means updated from server)
// This is going to loop over all properties in the client object...  need to find a more efficient way.
func (g *Game) ConvertFromObject(object *client.ClientObject) error {

	g.callbackCount++

	object.Lock.Lock()
	defer object.Lock.Unlock()
	for k, v := range object.Properties {

		// only care if property has been updated from server.
		if v.Updated {
			sp := strings.Split(k, "-")
			if len(sp) != 2 {
				return fmt.Errorf("invalid property name: %s", k)
			}
			x, err := strconv.Atoi(sp[0])
			if err != nil {
				return fmt.Errorf("invalid X property value: %s", sp[0])
			}
			y, err := strconv.Atoi(sp[1])
			if err != nil {
				return fmt.Errorf("invalid property name: %s", sp[1])
			}

			p := image.Point{x, y}

			b := box{colour: color.RGBA{v.Data[0], v.Data[1], v.Data[2], v.Data[3]}, Point: p}
			g.object.boxUpdateLock.Lock()
			g.object.boxes[p] = b
			g.object.boxUpdateLock.Unlock()
		}
	}

	return nil
}

// ConvertToObject converts a clients object TO the internal object.
// It takes in an existing internal object (if one exists) and updates it with the new data.
// It returns a pointer to the internal object.
// If the existingObject is nil, then it creates a new one.
func (g *Game) ConvertToObject(objectID string, existingObject *client.ClientObject, clientObject any) (*client.ClientObject, error) {
	var obj *client.ClientObject
	if existingObject == nil {
		obj = &client.ClientObject{
			ObjectID: objectID,
		}
	} else {
		obj = existingObject
	}

	gameObject := clientObject.(*GameObject)

	boxes := gameObject.GetBoxes()
	for _, box := range boxes {
		propertyID := fmt.Sprintf("%d-%d", box.X, box.Y)
		data := []byte{box.colour.R, box.colour.G, box.colour.B, 255}
		obj.AdjustProperty(propertyID, data, true, false)
	}

	return obj, nil
}
