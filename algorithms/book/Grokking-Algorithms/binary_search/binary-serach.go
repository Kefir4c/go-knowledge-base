package main

import (
	"fmt"
)

func main() {
	var arr = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	bin, count := binarySearch(arr, 10)

	fmt.Printf("Индекс числа 10:%d, Количество итераций: %d", bin, count)
}

func binarySearch(arr []int, val int) (int, int) {
	var count int
	low := 0
	height := len(arr) - 1

	for low <= height {
		count++
		mid := (low + height) / 2
		guesse := arr[mid]

		switch {
		case guesse == val:
			return mid, count

		case guesse < val:
			low = mid + 1

		case guesse > val:
			height = mid - 1
		}
	}
	return -1, 0
}
