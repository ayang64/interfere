package main

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Point struct {
	X, Y, W float64 // x, y, and wavelength
	DX, DY  float64 // x and y displacement
}

type ColorMapFunc func(float64, float64, float64) (byte, byte, byte)
type Interferer struct {
	ColorMapFunc
	Point []Point
}

func generatePoints(points int) []Point {
	rc := []Point{}
	for i := 0; i < points; i++ {
		rc = append(rc,
			Point{
				X:  rand.Float64(),
				Y:  rand.Float64(),
				W:  rand.Float64() * .2,
				DX: (rand.Float64() - .5) * .01,
				DY: (rand.Float64() - .5) * .02})
	}
	return rc
}

// builds and returns a new Interferer
func New(points int, cmapname string) (Interferer, error) {
	colmap := map[string]ColorMapFunc{
		"roygbiv": MapRoygbiv,
		"red":     MapRed,
		"bluered": MapBlueRed,
		"grey":    MapGrey,
	}

	m, exists := colmap[cmapname]

	if exists == false {
		return Interferer{}, fmt.Errorf("%s is not the name of a valid color mapping function.", cmapname)
	}

	rc := Interferer{
		ColorMapFunc: m,
		Point:        generatePoints(points),
	}

	return rc, nil
}

func (p *Point) Move() {
	p.X, p.Y = func() (float64, float64) {
		p.X += p.DX
		p.Y += p.DY

		// if a point moves off of our grid, wrap it around to the other
		// side.  i'm not sure if i like this better than bouncing.
		switch {
		case p.X < 0:
			p.X += 1.0
		case p.X > 1.0:
			p.X -= 1.0
		}

		switch {
		case p.Y < 0:
			p.Y += 1.0
		case p.Y > 1.0:
			p.Y -= 1.0
		}

		return p.X + p.DX, p.Y + p.DY
	}()
}

func (intf *Interferer) Update() {
	for i := range intf.Point {
		intf.Point[i].Move()
	}
}

func MapBlueRed(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange
	b := byte(wl * 255.0)
	return b, 0, 255 - b
}

func MapRed(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange
	b := byte(wl * 255.0)
	return b, 0, 0
}

func MapGrey(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange

	b := byte(wl * 255.0)

	return b, b, b
}

// from http://www.physics.sfasu.edu/astro/color/spectra.html
// scale value to color between 380nm and 780nm
func MapRoygbiv(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange

	wl = 380.0 + wl*400.0 // fit value between 380.0nm and 780.0nm

	var r, g, b float64

	switch {
	case wl < 380.0:
		r, g, b = 0.0, 0.0, 0.0
	case wl <= 440.0:
		r = -1.0 * (wl - 440.0) / (440.0 - 380.0)
		g = 0
		b = 1.0
	case wl <= 490.0:
		r = 0.0
		g = (wl - 440.0) / (490.0 - 440.0)
		b = 1.0
	case wl <= 510.0:
		r = 0.0
		g = 1.0
		b = -1.0 * (wl - 510.0) / (510.0 - 490.0)
	case wl <= 580.0:
		r = (wl - 510.0) / (580.0 - 510.0)
		g = 1.0
		b = 0.0
	case wl <= 645.0:
		r = 1.0
		g = -1 * (wl - 645.0) / (645.0 - 580.0)
		b = 0.0
	case wl <= 780.0:
		r = 1.0
		g = 0.0
		b = 0.0
	default:
		r, g, b = 0.0, 0.0, 0.0 // Should never happen
	}
	return byte(r * 255.0), byte(g * 255.0), byte(b * 255.0)
}

func SetForegroundRGB(s string, r, g, b byte) []byte {
	return []byte(fmt.Sprintf("\x1b[48;2;%d;%d;%dm%s", r, g, b, s))
}

func (intf *Interferer) Render(w, h int) {
	intf.Update()
	intf.Draw(w, h)
}

func (intf *Interferer) Draw(w, h int) {
	grid := make([]float64, w*h)

	gmax := -math.MaxFloat64
	gmin := math.MaxFloat64

	// try to allocate as close to our needed size up front.  each point
	// generated could require up to 22 characters + 6 for the cursor movement
	// escape characters
	bs := make([]byte, (w*h*22)+6)
	buf := bytes.NewBuffer(bs)

	// prepend escape code to move cursor to upper left hand corner to the
	// beginning of our output buffer.
	buf.Write([]byte("\x1b[1;1f"))

	var pr, pg, pb byte

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := x + y*w // position in array is x + y * stride

			for _, p := range intf.Point {
				px := p.X*float64(w) - float64(x)
				py := p.Y*float64(h) - float64(y)
				grid[a] += math.Sin(math.Hypot(px, py) * p.W)
			}

			if grid[a] > gmax {
				gmax = grid[a]
			}
			if grid[a] < gmin {
				gmin = grid[a]
			}

			// lets not set the color if we don't have to.
			r, g, b := intf.ColorMapFunc(grid[a], gmin, gmax)
			if pr == r && pg == g && pb == b {
				buf.Write([]byte{' '})
			} else {
				pr, pg, pb = r, g, b
				buf.Write(SetForegroundRGB(" ", r, g, b))
			}
		}
	}
	io.Copy(os.Stdout, buf)
}

func main() {
	rand.Seed(time.Now().Unix())

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGINT)

	w, h, _ := terminal.GetSize(0)

	points := flag.Int("points", 10, "Number of points to plot.")
	cmap := flag.String("cmap", "roygbiv", "Color map function to apply.  Options are: roygbiv, red, bluered, and grey.")
	flag.Parse()

	intf, err := New(*points, *cmap)

	if err != nil {
		log.Fatal(err)
	}

mainloop:
	for {
		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGWINCH:
				w, h, _ = terminal.GetSize(0)
			case syscall.SIGINT:
				break mainloop
			}
		case <-time.Tick(10 * time.Millisecond):
			intf.Render(w, h)
		}
	}
	// reset terminal on exit.
	fmt.Printf("\x1bc;")
}
