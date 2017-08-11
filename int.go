// for best results, run in a local terminal that supports 256 colors -- not
// over ssh -- and without a multiplexer like screen or tmux.
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
	Char          []byte  // character to print in each cell.
	Point         []Point // map of points where ripple originates.
	ColorMapFunc          // function to call when mapping a cell's value to a color.
	*bytes.Buffer         // buffer to store grid that we display
}

// return a slice of points primed with sane random values.
func generatePoints(points int) []Point {
	rc := []Point{}
	for i := 0; i < points; i++ {
		rc = append(rc,
			Point{
				X:  rand.Float64(),
				Y:  rand.Float64(),
				W:  rand.Float64() * .2,
				DX: (rand.Float64() - .5) * .01,
				DY: (rand.Float64() - .5) * .02,
			})
	}
	return rc
}

// builds and returns a new Interferer
func New(points int, cmapname string, char []byte) (*Interferer, error) {
	if len(char) == 0 {
		return nil, fmt.Errorf("char slice must be at least 1 byte long!")
	}

	colmap := map[string]ColorMapFunc{
		"roygbiv": MapRoygbiv,
		"red":     MapRed,
		"bluered": MapBlueRed,
		"grey":    MapGrey,
	}

	m, exists := colmap[cmapname]

	if exists == false {
		return nil, fmt.Errorf("%s is not the name of a valid color mapping function.", cmapname)
	}

	rc := &Interferer{
		ColorMapFunc: m,
		Point:        generatePoints(points),
		Char:         char,
		Buffer:       bytes.NewBuffer([]byte{}),
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
		// should never happen.  if it does, it means the either min_z or  max_z
		// values are wrong.
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
		// should never happen.  if it does, it means the either min_z or  max_z
		// values are wrong.
		r, g, b = 0.0, 0.0, 0.0
	}
	return byte(r * 255.0), byte(g * 255.0), byte(b * 255.0)
}

func SetForegroundRGB(c []byte, r, g, b byte) []byte {
	return []byte(fmt.Sprintf("\x1b[48;2;%d;%d;%dm%s", r, g, b, c))
}

func (intf *Interferer) Render(w, h int) {
	intf.Update()   // update point positions
	intf.Draw(w, h) // compute grid and write it to the screen.
}

func (intf *Interferer) Draw(w, h int) {
	grid := make([]float64, w*h)

	// prepend escape code to move cursor to upper left hand corner to the
	// beginning of our output buffer.
	intf.Buffer.Write([]byte("\x1b[1;1f"))

	// prime gmin and gmax with ±infinity. by the end of the loops below they
	// will contain the minimum and maximum value set in our output grid.  we fit
	// the color for each element into a range between gmin and gmax.
	gmax := math.Inf(-1)
	gmin := math.Inf(1)

	// store a float version of the widh and height to avoid type conversion inside
	// our loop.  i'm not even sure if this helps -- maybe the compiler might be smart
	// enough to do this itself. *shrug*
	fw, fh := float64(w), float64(h)

	// our main loop.  SUBTLE: pr, pg, and pb are declared in this loop because
	// we don't need it outide of that scope.
	for y, pr, pg, pb := 0, byte(0), byte(0), byte(0); y < h; y++ {
		for x := 0; x < w; x++ {
			a := x + y*w // position in array is x + y * stride

			// hoist type conversion of x and y to float64 out of the loop below.
			fx, fy := float64(x), float64(y)
			for _, p := range intf.Point {
				px := p.X*fw - fx
				py := p.Y*fh - fy
				grid[a] += math.Sin(math.Hypot(px, py) * p.W)
			}

			// update max and min values we've seen so far
			gmax = math.Max(grid[a], gmax)
			gmin = math.Min(grid[a], gmin)

			// map current value to a color.
			r, g, b := intf.ColorMapFunc(grid[a], gmin, gmax)

			// lets not set the color if we don't have to.
			//
			// we've stored the previous r, g, b values in pr, pg, pb (previous-r,
			// etc...).  if the new ones match the previous, we just append a
			// character to our output buffer.  otherwise, we append the escape
			// characters to set the color *and* the new character.
			if pr == r && pg == g && pb == b {
				intf.Buffer.Write(intf.Char)
			} else {
				pr, pg, pb = r, g, b
				intf.Buffer.Write(SetForegroundRGB(intf.Char, r, g, b))
			}
		}
	}
	io.Copy(os.Stdout, intf.Buffer)
}

func main() {
	rand.Seed(time.Now().Unix())

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGINT)

	cmap := flag.String("cmap", "roygbiv", "Color map function to apply.  Options are: roygbiv, red, bluered, and grey.")
	char := flag.String("char", " ", "Character to print in each 'pixel'. ")
	points := flag.Int("points", 10, "Number of points to plot.")
	flag.Parse()

	intf, err := New(*points, *cmap, []byte(*char))

	if err != nil {
		log.Fatalf("error: %s", err)
	}

	// prime our temrminal size.
	w, h, _ := terminal.GetSize(0)
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
