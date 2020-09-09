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
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ayang64/asciiart"
	"golang.org/x/crypto/ssh/terminal"
)

type Point struct {
	P complex128 // point coordinates
	D complex128 // delta in each axis
	W float64    // wavelength
}

type ColorMapFunc func(float64, float64, float64) color.RGBA
type Interferer struct {
	Point         []Point // map of points where ripple originates.
	ColorMapFunc          // function to call when mapping a cell's value to a color.
	*bytes.Buffer         // buffer to store grid that we display
	GoRoutines    int     // number of goroutines to spawn.
	GridMutex     sync.Mutex
	Grid          []float64 // resulting grid
	Width         int       // width of display grid
	Height        int       // height of display grid
	Image         *image.RGBA
	dims          chan [2]int
	done          chan [2]float64
	colorbuf      []byte
}

func (intf *Interferer) SetDimensions(w, h int) error {
	intf.GridMutex.Lock()
	intf.Image = image.NewRGBA(image.Rect(0, 0, w, h))
	intf.Width, intf.Height = w, h
	intf.Grid = make([]float64, intf.Width*intf.Height)
	intf.GridMutex.Unlock()
	return nil
}

// return a slice of points primed with sane random values.
func generatePoints(points int) []Point {
	rc := make([]Point, points)

	// the actual coordinates we use are floats and we're bouncing the points
	// around a 1.0 x 1.0 field.  later we translate this grid to terminal
	// coordinates.
	for i := 0; i < points; i++ {
		rc[i] = Point{
			D: complex(((rand.Float64() - .5) * .01), ((rand.Float64() - .5) * .02)),
			P: complex(rand.Float64(), rand.Float64()),
			W: rand.Float64() * .2,
		}
	}
	return rc
}

// builds and returns a new Interferer
func New(points int, w int, h int, cmapname string, goroutines int) (*Interferer, error) {
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
		Buffer:       bytes.NewBuffer([]byte{}),
		GoRoutines:   goroutines,
		dims:         make(chan [2]int),
		colorbuf:     make([]byte, 256),
	}

	rc.SetDimensions(w, h)

	return rc, nil
}

func (p *Point) Move() {
	// if a point moves off of our grid, wrap it around to the other
	// side.  i'm not sure if i like this better than bouncing.
	n := p.P + p.D

	if real(n) < 0.0 || real(n) > 1.0 {
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

func MapBlueRed(z, min_z, max_z float64) color.RGBA {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange
	b := int(wl * 255.0)
	return color.RGBA{R: uint8(b), G: 0, B: 255 - uint8(b), A: 255}
}

func MapRed(z, min_z, max_z float64) color.RGBA {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange
	b := int(wl * 255.0)
	return color.RGBA{R: uint8(b), G: 0, B: 0, A: 255}
}

func MapLorn(z, min_z, max_z float64) color.RGBA {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange

	b := func() int {
		if wl > .60 {
			return 0xdf
		}
		return 0x0
	}()
	return color.RGBA{R: uint8(b), G: uint8(b), B: uint8(b), A: 255}
}

func MapGrey(z, min_z, max_z float64) color.RGBA {
	zrange := max_z - min_z
	absz := z - min_z
	wl := absz / zrange
	b := int(wl * 255.0)
	return color.RGBA{R: uint8(b), G: uint8(b), B: uint8(b), A: 255}
}

// from http://www.physics.sfasu.edu/astro/color/spectra.html
// scale value to color between 380nm and 780nm
func MapRoygbiv(z, min_z, max_z float64) color.RGBA {
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
	return color.RGBA{R: uint8(r * 255.0), G: uint8(g * 255.0), B: uint8(b * 255.0), A: 255}
}

func SetForegroundRGB(r, g, b int) []byte {
	return []byte("\x1b[48;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m")
}

func (intf *Interferer) Render(ctx context.Context) {
	intf.Update() // update point positions

	intf.GridMutex.Lock()
	min, max := intf.Compute() // compute grid and write it to the screen.
	intf.Draw(min, max)        // compute grid and write it to the screen.
	intf.GridMutex.Unlock()
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
	if intf.done == nil || cap(intf.done) < goroutines {
		intf.done = make(chan [2]float64, goroutines)
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
					intf.Grid[a] = 0.0
					for idx := range intf.Point {
						intf.Grid[a] += math.Sin(math.Hypot(real(intf.Point[idx].P)*fw-fx, imag(intf.Point[idx].P)*fh-fy) * intf.Point[idx].W)
					}
					// update max and min values we've seen so far
					localmin = math.Min(intf.Grid[a], localmin)
					localmax = math.Max(intf.Grid[a], localmax)
				}
			}
			intf.done <- [2]float64{localmin, localmax}
		}(starth, maxh)
	}

	gmin, gmax := math.Inf(1), math.Inf(-1)

	for i := 0; i < goroutines; i++ {
		mm := <-intf.done
		gmin, gmax = math.Min(gmin, mm[0]), math.Max(gmax, mm[1])
	}

	return gmin, gmax
}

func (intf *Interferer) Draw(gmin, gmax float64) error {
	// prepend escape code to move cursor to upper left hand corner to the
	// beginning of our output buffer.
	for y := 0; y < intf.Height; y++ {
		for x := 0; x < intf.Width; x++ {
			a := x + y*intf.Width // position in array is x + y * stride
			intf.Image.Set(x, y, intf.ColorMapFunc(intf.Grid[a], gmin, gmax))
		}
	}

	if err := asciiart.Encode(os.Stdout, intf.Image); err != nil {
		return err
	}
	return nil
}

func run(ctx context.Context) float64 {
	cmap := flag.String("cmap", "roygbiv", "Color map function to apply.  Options are: roygbiv, red, bluered, and grey.")
	points := flag.Int("points", 10, "Number of points to plot.")
	goroutines := flag.Int("goroutines", runtime.NumCPU(), "Number of goroutines to spawn when creating grid. Defaults to number of logical CPUs.")
	traceFile := flag.String("trace", "", "File to output trace information to. If empty, then no trace information is saved.")
	runDuration := flag.Duration("duration", time.Duration(0), "Max run time in seconds.")
	memprofile := flag.String("memprofile", "", "Location of memory profile.")
	flag.Parse()

	ctx, timeOutCancel := func() (context.Context, func()) {
		if *runDuration != time.Duration(0) {
			return context.WithTimeout(ctx, *runDuration)
		}
		return ctx, func() {}
	}()

	defer timeOutCancel()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if *traceFile != "" {
		w, err := os.Create(*traceFile)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Close()

		if err := trace.Start(w); err != nil {
			log.Fatal(err)
		}

		defer trace.Stop()
	}

	w, h, _ := terminal.GetSize(0)
	intf, err := New(*points, w, h*2, *cmap, *goroutines)

	if err != nil {
		log.Fatalf("error: %s", err)
	}

	start := time.Now()
	renders := 0
	go func() {
		for {
			if err := ctx.Err(); err != nil {
				// cancelled
				return
			}
			intf.Render(ctx)
			renders++
		}
	}()

	// restore terminal before returning.
	defer fmt.Printf("\x1bc")

	// handle sigwinch and sigint
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGINT)

	go func() {
		for dim := range intf.dims {
			intf.SetDimensions(dim[0], dim[1])
		}
	}()

mainloop:
	for {
		select {
		case <-ctx.Done():
			break mainloop
		case sig := <-sigs:
			switch sig {
			case syscall.SIGWINCH:
				w, h, _ := terminal.GetSize(0)
				// send new dimensions to renderer
				intf.dims <- [2]int{w, h * 2}
			case syscall.SIGINT:
				break mainloop
			}
		}
	}

	cancel()

	duration := time.Since(start)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)

		if err != nil {
			log.Fatal(err)
		}

		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal(err)
		}

		f.Close()
	}

	return float64(renders) / duration.Seconds()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	fmt.Printf("%f frames a second.\n", run(context.Background()))
}
