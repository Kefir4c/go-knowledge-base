package main

import (
	"fmt"
	"slices"
)

func main() {
	fmt.Println(selectionSort([]int{2, 3, 1, 4, 5, 12}))
}

func findSmallest(arr []int) int {
	smallest := arr[0]
	smallesIndex := 0

	for i := 1; i < len(arr); i++ {
		if arr[i] < smallest {
			smallest = arr[i]
			smallesIndex = i
		}
	}
	return smallesIndex
}

func selectionSort(arr []int) []int {
	newArr := slices.Clone(arr)

	for i := 0; i < len(newArr); i++ {
		smallest := findSmallest(newArr[i:]) + i
		newArr[i], newArr[smallest] = newArr[smallest], newArr[i]
	}
	return newArr
}
