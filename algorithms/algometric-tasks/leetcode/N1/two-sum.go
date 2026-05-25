package main

import "fmt"

func main() {
	fmt.Println(twoSum([]int{1, 3, 5, 7, 1, 3, 2, 9, 10, 4}, 10))
}

func twoSum(nums []int, target int) []int {
	m := make(map[int]int, len(nums))

	for ind, val := range nums {
		needed := target - val

		if secondInd, ok := m[needed]; ok {
			return []int{secondInd, ind}
		}

		m[val] = ind
	}
	return nil
}
