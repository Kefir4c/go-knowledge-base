package main

import (
	"testing"
)

const (
	rows = 2000
	cols = 2000
)

func createMatrix() [rows][cols]int {
	return [rows][cols]int{}
}

func BenchmarkRowMajor(b *testing.B) {
	matrix := createMatrix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				matrix[r][c] = i
			}
		}
	}
}
func BenchmarkColumnMajor(b *testing.B) {
	matrix := createMatrix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for c := 0; c < cols; c++ {
			for r := 0; r < rows; r++ {
				matrix[r][c] = i
			}
		}
	}
}
