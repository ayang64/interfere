/*
	for best results, run in a local terminal that supports 256 colors -- not
	over ssh -- and without a multiplexer like screen or tmux.

	i noticed francesc ask about terminal colors on twitter.  i decided to try
	my hand at a terminal based 'demo' that is imspired by some stuff i used to
	see in my amiga days.

	enjoy!
	ayan@ayan.net
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/trace"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

type Point struct {
	P complex128 // point coordinates
	D complex128 // delta in each axis
	W float64    // x, y, and wavelength
}

type ColorMapFunc func(float64, float64, float64) (byte, byte, byte)
type Interferer struct {
	Message       []byte    // characters to print in each cell.
	Point         []Point   // map of points where ripple originates.
	ColorMapFunc            // function to call when mapping a cell's value to a color.
	*bytes.Buffer           // buffer to store grid that we display
	GoRoutines    int       // number of goroutines to spawn.
	Grid          []float64 // resulting grid
	Width         int       // width of display grid
	Height        int       // height of display grid
}

func (intf *Interferer) SetDimensions(w, h int) error {
	intf.Width, intf.Height = w, h
	intf.Grid = make([]float64, intf.Width*intf.Height)
	return nil
}

// return a slice of points primed with sane random values.
func generatePoints(points int) []Point {
	rc := make([]Point, points)

	// the actual coordinates we use are floats and we're bouncing the points
	// around a 1.0 x 1.0 field.  later we translate this grid to terminal
	// coordinates.
	for i := 0; i < points; i++ {
		rc = append(rc,
			Point{
				D: complex(((rand.Float64() - .5) * .01), ((rand.Float64() - .5) * .02)),
				P: complex(rand.Float64(), rand.Float64()),
				W: rand.Float64() * .2,
			})
	}
	return rc
}

// builds and returns a new Interferer
func New(points int, w int, h int, cmapname string, message []byte, goroutines int) (*Interferer, error) {
	colmap := map[string]ColorMapFunc{
		"roygbiv": MapRoygbiv,
		"lorn":    MapLorn,
		"red":     MapRed,
		"bluered": MapBlueRed,
		"grey":    MapGrey,
	}

	if _, exists := colmap[cmapname]; exists == false {
		return nil, fmt.Errorf("%s is not the name of a valid color mapping function.", cmapname)
	}

	rc := &Interferer{
		ColorMapFunc: colmap[cmapname],
		Point:        generatePoints(points),
		Message:      message,
		Buffer:       bytes.NewBuffer([]byte{}),
		GoRoutines:   goroutines,
	}

	rc.SetDimensions(w, h)

	return rc, nil
}

func (p *Point) Move() {
	// if a point moves off of our grid, wrap it around to the other
	// side.  i'm not sure if i like this better than bouncing.
	n := p.P + p.D

	if real(n) < 0.0 || real(n) > 1.0 {
		// p.D *= complex(-1, 1)
		p.D = complex(-real(p.D), imag(p.D))
	}

	if imag(n) < 0.0 || imag(n) > 1.0 {
		p.D = complex(real(p.D), -imag(p.D))
	}

	p.P += p.D
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

func MapLorn(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange

	b := func() byte {
		if wl > .60 {
			return 0xdf
		}
		return 0x0
	}()
	return b, b, b
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
		// should never happen.  if it does, it means the either min_z or max_z
		// values are wrong.
		r, g, b = 0.0, 0.0, 0.0
	case wl <= 440.0:
		r, g, b = -1.0*(wl-440.0)/(440.0-380.0), 0.0, 1.0
	case wl <= 490.0:
		r, g, b = 0.0, (wl-440.0)/(490.0-440.0), 1.0
	case wl <= 510.0:
		r, g, b = 0.0, 1.0, -1.0*(wl-510.0)/(510.0-490.0)
	case wl <= 580.0:
		r, g, b = (wl-510.0)/(580.0-510.0), 1.0, 0.0
	case wl <= 645.0:
		r, g, b = 1.0, -1*(wl-645.0)/(645.0-580.0), 0.0
	case wl <= 780.0:
		r, g, b = 1.0, 0.0, 0.0
	default:
		// should never happen.  if it does, it means the either min_z or max_z
		// values are wrong.
		r, g, b = 0.0, 0.0, 0.0
	}
	return byte(r * 255.0), byte(g * 255.0), byte(b * 255.0)
}

func SetForegroundRGB(r, g, b byte) []byte {
	return []byte(fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b))
}

func (intf *Interferer) Render() {
	intf.Update()              // update point positions
	min, max := intf.Compute() // compute grid and write it to the screen.
	intf.Draw(min, max)        // compute grid and write it to the screen.
}

func (intf *Interferer) Compute() (float64, float64) {
	// store a float version of the widh and height to avoid type conversion
	// inside our loop.  i'm not even sure if this helps -- maybe the compiler is
	// smart enough to do this itself. *shrug*
	fw, fh := float64(intf.Width), float64(intf.Height)

	// loop through points on screen and add up sin( distance to point[n] ) *
	// frequency this will make a nice swirly/wavy pattern where the functions
	// add up constructively.

	goroutines := func() int {
		if intf.GoRoutines > intf.Height {
			return intf.Height
		}
		return intf.GoRoutines
	}()

	lines := intf.Height / goroutines
	done := make(chan [2]float64, goroutines)

	for i := range intf.Grid {
		intf.Grid[i] = 0.0
	}

	for g := 0; g < goroutines; g++ {
		starth, maxh := func() (int, int) {
			if g == goroutines-1 {
				return g * lines, intf.Height
			}
			return g * lines, (g + 1) * lines
		}()

		go func(hstart, hend int) {
			// prime gmin and gmax with ±infinity. by the end of the loops below they
			// will contain the minimum and maximum value set in our output grid.  we fit
			// the color for each element into a range between gmin and gmax.
			localmin, localmax := math.Inf(1), math.Inf(-1)

			for y := hstart; y < hend; y++ {
				for x := 0; x < intf.Width; x++ {
					a := x + y*intf.Width // position in array is x + y * stride
					// hoist type conversion of x and y to float64 out of the loop below.
					fx, fy := float64(x), float64(y)
					for idx := range intf.Point {
						intf.Grid[a] += math.Sin(math.Hypot(real(intf.Point[idx].P)*fw-fx, imag(intf.Point[idx].P)*fh-fy) * intf.Point[idx].W)
					}
					// update max and min values we've seen so far
					localmin = math.Min(intf.Grid[a], localmin)
					localmax = math.Max(intf.Grid[a], localmax)
				}
			}
			done <- [2]float64{localmin, localmax}
		}(starth, maxh)
	}

	gmin, gmax := math.Inf(1), math.Inf(-1)

	for i := 0; i < goroutines; i++ {
		mm := <-done
		gmin, gmax = math.Min(gmin, mm[0]), math.Max(gmax, mm[1])
	}

	return gmin, gmax
}

func (intf *Interferer) Draw(gmin, gmax float64) {
	// prepend escape code to move cursor to upper left hand corner to the
	// beginning of our output buffer.
	intf.Buffer.Write([]byte("\x1b[;f"))

	// map each point in the grid to a color and append characters to our output
	// bufferr.
	//
	// SUBTLE: pr, pg, and pb are declared in this loop because we don't need
	// them outide of that scope.
	for a, pr, pg, pb := 0, byte(0), byte(0), byte(0); a < len(intf.Grid); a++ {
		// map current value to a color.
		r, g, b := intf.ColorMapFunc(intf.Grid[a], gmin, gmax)

		// lets not set the color if we don't need to.
		//
		// we've stored the previous r, g, b values in pr, pg, pb (previous-r,
		// etc...).  if the new ones match the previous, we simply append a
		// character to our output buffer as the color is already what we want.
		// otherwise, we append escape characters to set the color *and* the new
		// character.
		c := []byte{intf.Message[a%len(intf.Message)]}
		if pr != r || pg != g || pb != b {
			pr, pg, pb = r, g, b
			intf.Buffer.Write(SetForegroundRGB(r, g, b))
		}
		intf.Buffer.Write(c)
	}

	// at this point we should have a colorful buffer to push to the terminal!
	// lets copy it to os.Stdout.
	io.Copy(os.Stdout, intf.Buffer)
}

func run() float64 {
	cmap := flag.String("cmap", "roygbiv", "Color map function to apply.  Options are: roygbiv, red, bluered, and grey.")
	message := flag.String("message", " ", "Message to repeat on terminal. ")
	points := flag.Int("points", 10, "Number of points to plot.")
	goroutines := flag.Int("goroutines", runtime.NumCPU(), "Number of goroutines to spawn when creating grid. Defaults to number of logical CPUs.")
	traceFile := flag.String("trace", "", "File to output trace information to. If empty, then no trace information is saved.")
	flag.Parse()

	if *traceFile != "" {
		w, err := os.Create(*traceFile)

		if err != nil {
			log.Fatal(err)
		}

		if err := trace.Start(w); err != nil {
			log.Fatal(err)
		}

		defer trace.Stop()
	}

	w, h, _ := terminal.GetSize(0)
	intf, err := New(*points, w, h, *cmap, []byte(*message), *goroutines)

	if err != nil {
		log.Fatalf("error: %s", err)
	}

	rand.Seed(time.Now().Unix())

	dims := make(chan [2]int)

	start := time.Now()
	renders := 0
	go func() {
		for {
			select {
			case wh := <-dims:
				intf.SetDimensions(wh[0], wh[1])
			default:
				intf.Render()
				renders++
			}
		}
	}()

	// restore terminal before returning.
	defer fmt.Printf("\x1bc")

	// handle sigwinch and sigint
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGINT)

mainloop:
	for {
		sig := <-sigs
		switch sig {
		case syscall.SIGWINCH:
			w, h, _ := terminal.GetSize(0)
			// send new dimensions to renderer
			dims <- [2]int{w, h}

		case syscall.SIGINT:
			break mainloop
		}
	}

	duration := time.Since(start)
	return float64(renders) / duration.Seconds()
}

func main() {
	rand.Seed(time.Now().Unix())
	fmt.Printf("%f frames a second.\n", run())
}
