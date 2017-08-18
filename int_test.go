package main

import "testing"

func BenchmarkCompute(b *testing.B) {

	benchmark := []struct {
		Name       string
		Points     int
		GoRoutines int
		Width      int
		Height     int
	}{
		{Name: "1 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 1},
		{Name: "2 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 2},
		{Name: "4 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 4},
		{Name: "8 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 8},
		{Name: "16 goroutine", Points: 10, Width: 1000, Height: 1000, GoRoutines: 16},
		{Name: "32 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 32},
		{Name: "64 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 64},
		{Name: "50 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 50},
		{Name: "100 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 100},
		{Name: "500 goroutines", Points: 10, Width: 1000, Height: 1000, GoRoutines: 500},
	}

	for i := range benchmark {
		intf, _ := New(benchmark[i].Points, "roygbiv", []byte{' '}, benchmark[i].GoRoutines)
		b.Run(benchmark[i].Name, func(b *testing.B) {
			intf.Compute(benchmark[i].Width, benchmark[i].Height)
		})
	}
}
