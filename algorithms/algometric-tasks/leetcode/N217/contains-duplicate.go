package main

import "fmt"

func main() {
	fmt.Println(containsDuplicate([]int{1, 3, 5, 7, 2, 9, 10, 4}))
}

func containsDuplicate(nums []int) bool {
	if len(nums) < 1 {
		return false
	}

	arr := make(map[int]struct{}, len(nums))

	for _, val := range nums {
		if _, ok := arr[val]; ok {
			return true
		}
		arr[val] = struct{}{}
	}
	return false
}
