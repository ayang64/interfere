package main

import (
	"testing"
)

// benchmark rendering performance across a variety of goroutines.
func BenchmarkCompute(b *testing.B) {
	benchmark := []struct {
		Name       string
		Points     int
		GoRoutines int
		Width      int
		Height     int
	}{
		{Name: "001 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 1},
		{Name: "002 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 2},
		{Name: "003 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 3},
		{Name: "004 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 4},
		{Name: "008 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 8},
		{Name: "016 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 16},
		{Name: "032 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 32},
		{Name: "050 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 50},
		{Name: "064 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 64},
		{Name: "128 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 128},
		{Name: "256 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 256},
		{Name: "512 goroutines", Points: 10, Width: 512, Height: 512, GoRoutines: 512},
	}

	for _, bm := range benchmark {
		intf, _ := New(bm.Points, bm.Width, bm.Height, "roygbiv", bm.GoRoutines)
		b.ResetTimer()
		b.Run(bm.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				intf.Compute()
			}
		})
	}
}
