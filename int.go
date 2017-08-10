package main

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Point struct {
	X, Y, W float64 // x, y, and wavelength
	DX, DY  float64 // x and y velocity/displacement of point
}

type Interferer struct {
	Point []Point
}

func New(points int) Interferer {
	rc := Interferer{}

	rc.Init(points)

	return rc
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

// from http://www.physics.sfasu.edu/astro/color/spectra.html
// scale value to color between 380nm and 780nm
func full_spectrum(z, min_z, max_z float64) (byte, byte, byte) {
	zrange := float64(max_z) - min_z
	absz := z - min_z
	wl := absz / zrange

	wl = 380.0 + wl*400.0 // fit value between 380.0nm and 780.0nm

	var r, g, b float64

	switch {
	case wl < 380.0:
		r, g, b = 0.0, 0.0, 0.0
	case wl <= 440.0:
		g = 0
		r = -1.0 * (wl - 440.0) / (440.0 - 380.0)
		b = 1.0
	case wl <= 490.0:
		r = 0.0
		g = (wl - 440.0) / (490.0 - 440.0)
		b = 1.0
	case wl <= 510.0:
		g = 1.0
		r = 0.0
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

func ForegroundRGB(buf *bytes.Buffer, s byte, r, g, b byte) {
	buf.Write([]byte("\x1b[38;2;"))
	buf.Write([]byte(strconv.Itoa(int(r))))
	buf.Write([]byte{';'})
	buf.Write([]byte(strconv.Itoa(int(b))))
	buf.Write([]byte{';'})
	buf.Write([]byte(strconv.Itoa(int(g))))
	buf.Write([]byte{'m'})
	buf.Write([]byte{s})
}

func (intf *Interferer) Render(w, h int) {
	intf.Update()
	intf.Draw(w, h)
}

func (intf *Interferer) Draw(w, h int) {
	grid := make([]float64, w*h)

	gmax := -math.MaxFloat64
	gmin := math.MaxFloat64

	b := []byte{}

	buf := bytes.NewBuffer(b)

	// move cursor to upper left hand corner.
	fmt.Printf("\x1b[%d;%df", 0, 0)
	var prev float64

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := x + y*w // x + y * stride
			grid[a] = 0.0
			var v float64
			for _, p := range intf.Point {
				px := p.X * float64(w)
				py := p.X * float64(h)
				v += math.Sin(math.Hypot(px-float64(x), py-float64(y)) * p.W)
			}
			grid[a] += v

			if grid[a] > gmax {
				gmax = grid[a]
			}
			if grid[a] < gmin {
				gmin = grid[a]
			}

			// lets not set the color if we don't have to.
			if grid[a] != prev {
				r, g, b := full_spectrum(grid[a], gmin, gmax)
				ForegroundRGB(buf, '*', r, g, b)
				prev = grid[a]
			} else {
				buf.Write([]byte{'*'})
			}
		}
	}
	io.Copy(os.Stdout, buf)
}

func (intf *Interferer) Init(points int) {
	for i := 0; i < points; i++ {
		intf.Point = append(intf.Point,
			Point{
				X:  rand.Float64(),
				Y:  rand.Float64(),
				W:  rand.Float64() * .2,
				DX: (rand.Float64() - .5) * .01,
				DY: (rand.Float64() - .5) * .02})
	}
}

func main() {
	rand.Seed(time.Now().Unix())

	winch := make(chan os.Signal)
	signal.Notify(winch, syscall.SIGWINCH)

	w, h, _ := terminal.GetSize(0)

	points := flag.Int("points", 10, "Number of points to plot.")
	flag.Parse()

	intf := New(*points)

	for {
		select {
		case <-winch:
			w, h, _ = terminal.GetSize(0)
		case <-time.Tick(10 * time.Millisecond):
			intf.Render(w, h)
		}
	}
}
