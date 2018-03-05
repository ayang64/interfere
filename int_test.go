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
		{Name: "1 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 1},
		{Name: "2 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 2},
		{Name: "3 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 3},
		{Name: "4 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 4},
		{Name: "8 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 8},
		{Name: "16 goroutine", Points: 10, Width: 100, Height: 100, GoRoutines: 16},
		{Name: "32 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 32},
		{Name: "50 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 50},
		{Name: "64 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 64},
		{Name: "100 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 100},
		{Name: "500 goroutines", Points: 10, Width: 100, Height: 100, GoRoutines: 500},
	}

	for _, bm := range benchmark {
		intf, _ := New(bm.Points, bm.Width, bm.Height, "roygbiv", []byte{' '}, bm.GoRoutines)
		b.ResetTimer()
		b.Run(bm.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				intf.Compute()
			}
		})
	}
}
