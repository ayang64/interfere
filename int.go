package main

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
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
	DX, DY  float64 // x and y velocity of point
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
		/*
			if p.X+p.DX < 0 || p.X+p.DX > 1.0 {
				p.DX *= -1
			}
			if p.Y+p.DY < 0 || p.Y+p.DY > 1.0 {
				p.DY *= -1
			}
		*/

		p.X += p.DX
		p.Y += p.DY

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
	case wl < 440.0:
		r, g, b = 0.0, 0.0, 0.0 // Should never happen
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

func ForegroundRGB(s string, r, g, b byte) string {
	return "\x1b[38;2;" + strconv.Itoa(int(r)) + ";" + strconv.Itoa(int(b)) + ";" + strconv.Itoa(int(g)) + "m" + s
}

func (intf *Interferer) Draw(w, h int) {
	grid := make([]float64, w*h)

	gmax := -math.MaxFloat64
	gmin := math.MaxFloat64

	fmt.Printf("\x1b[%d;%df", 1, 1)
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

			r, g, b := full_spectrum(grid[a], gmin, gmax)
			// fmt.Printf("x = %d, y = %d, a = %d, %v,%v,%v\n", x, y, a, r, g, b)
			// fmt.Printf("%s", ForegroundRGB("◼", r, g, b))
			fmt.Printf("%s", ForegroundRGB(".", r, g, b))
		}
	}
}

func (intf *Interferer) Init(points int) {
	for i := 0; i < points; i++ {
		intf.Point = append(intf.Point, Point{X: rand.Float64(), Y: rand.Float64(), W: rand.Float64() * .5, DX: (rand.Float64() - .5) * .02, DY: (rand.Float64() - .5) * .02})
	}
}

func main() {

	rand.Seed(time.Now().Unix())
	winch := make(chan os.Signal)

	signal.Notify(winch, syscall.SIGWINCH)

	w, h, _ := terminal.GetSize(0)

	i := 0

	intf := New(3)

	for {
		select {
		case <-winch:
			w, h, _ = terminal.GetSize(0)
			i++
		case <-time.Tick(10 * time.Millisecond):
			intf.Update()
			intf.Draw(w, h)
		}
	}
}
