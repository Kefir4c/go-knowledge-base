package main

import (
	"fmt"
)

func main() {
	fmt.Println(sum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursinSum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursinSumInd([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursionMaxNum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursionMax([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recBinSearch([]int{2, 6, 5, 2, 1, 9}, 5, 0, 5))
}

func sum(sl []int) int {
	var result int
	for _, val := range sl {
		result += val
	}
	return result
}

func recursinSum(sl []int) int {
	if len(sl) == 0 {
		return 0
	}

	return sl[0] + recursinSum(sl[1:])
}

func recursinSumInd(sl []int) int {
	if len(sl) == 0 {
		return 0
	}

	return 1 + recursinSumInd(sl[1:])
}

func recursionMaxNum(sl []int) int {
	if len(sl) == 1 {
		return sl[0]
	}
	if sl[0] > sl[1] {
		sl[1] = sl[0]
		return recursionMaxNum(sl[1:])
	}
	return recursionMaxNum(sl[1:])
}

func recursionMax(sl []int) int {
	if len(sl) == 1 {
		return sl[0]
	}

	subMax := recursionMax(sl[1:])

	if sl[0] > subMax {
		return sl[0]
	}
	return subMax
}

func recBinSearch(sl []int, val int, low int, high int) int {
	if low > high {
		return -1
	}

	mid := (low + high) / 2
	if sl[mid] == val {
		return mid
	}
	if sl[mid] > val {
		return recBinSearch(sl, val, low, mid-1)
	} else {
		return recBinSearch(sl, val, mid+1, high)
	}
}
