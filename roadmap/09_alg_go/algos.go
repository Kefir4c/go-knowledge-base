package _9_alg_go

import (
	"sort"
	"strconv"
)

// Spiral Matrix
func spiralOrder(matrix [][]int) []int {
	if len(matrix) == 0 {
		return []int{}
	}
	res := []int{}

	top, bottom := 0, len(matrix)-1
	left, right := 0, len(matrix[0])-1

	for left <= right && top <= bottom {

		for i := left; i <= right; i++ {
			res = append(res, matrix[top][i])
		}
		top++

		for i := top; i <= bottom; i++ {
			res = append(res, matrix[i][right])
		}
		right--

		if top > bottom {
			break
		}

		for i := right; i >= left; i-- {
			res = append(res, matrix[bottom][i])
		}
		bottom--

		if left > right {
			break
		}

		for i := bottom; i >= top; i-- {
			res = append(res, matrix[i][left])
		}
		left++

	}
	return res
}

// Product of Array Except Self
func productExceptSelf(nums []int) []int {
	n := len(nums)
	res := make([]int, n)

	res[0] = 1
	for i := 1; i < n; i++ {
		res[i] = res[i-1] * nums[i-1]
	}

	rightProd := 1
	for i := n - 1; i >= 0; i-- {
		res[i] = res[i] * rightProd
		rightProd = rightProd * nums[i]
	}
	return res
}

// Subarray Sum Equals K
func subarraySum(nums []int, k int) int {
	prefixSum := map[int]int{0: 1}
	count, currSum := 0, 0

	for _, num := range nums {
		currSum += num

		if freq, ok := prefixSum[currSum-k]; ok {
			count += freq
		}
		prefixSum[currSum]++
	}
	return count
}

// Container With Most Water
func maxArea(height []int) int {
	maxArea := 0
	left, right := 0, len(height)-1

	for left < right {
		min := height[left]
		if height[left] > height[right] {
			min = height[right]
		}

		area := (right - left) * min

		if area > maxArea {
			maxArea = area
		}

		if height[left] < height[right] {
			left++
		} else {
			right--
		}
	}
	return maxArea
}

// 3Sum
func threeSum(nums []int) [][]int {
	n := len(nums)
	sort.Ints(nums)
	res := [][]int{}

	for i := 0; i < n-2; i++ {

		if i > 0 && nums[i] == nums[i-1] {
			continue
		}

		left := i + 1
		right := n - 1

		for left < right {
			sum := nums[i] + nums[left] + nums[right]

			if sum == 0 {
				res = append(res, []int{nums[i], nums[left], nums[right]})

				for left < right && nums[left] == nums[left+1] {
					left++
				}
				for left < right && nums[right] == nums[right-1] {
					right--
				}
				left++
				right--
			} else if sum < 0 {
				left++
			} else {
				right--
			}
		}
	}
	return res
}

// Minimum Size Subarray Sum
func minSubArrayLen(target int, nums []int) int {
	n := len(nums)
	minLen := n + 1
	left, currSum := 0, 0

	for right := 0; right < n; right++ {
		currSum += nums[right]

		for currSum >= target {

			if right-left+1 < minLen {
				minLen = right - left + 1
			}

			currSum -= nums[left]
			left++
		}
	}
	if minLen == n+1 {
		return 0
	}
	return minLen
}

// Daily Temperatures
func dailyTemperatures(temperatures []int) []int {
	n := len(temperatures)
	res := make([]int, n)
	stack := []int{}

	for i := 0; i < n; i++ {
		for len(stack) > 0 && temperatures[i] > temperatures[stack[len(stack)-1]] {
			curr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			res[curr] = i - curr

		}
		stack = append(stack, i)
	}
	return res
}

// Evaluate Reverse Polish Notation
func evalRPN(tokens []string) int {
	stack := []int{}

	for _, token := range tokens {
		if token == "+" || token == "-" || token == "/" || token == "*" {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			a := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			var result int
			switch token {
			case "+":
				result = a + b
			case "-":
				result = a - b
			case "*":
				result = a * b
			case "/":
				result = a / b
			}
			stack = append(stack, result)
		} else {
			num, _ := strconv.Atoi(token)
			stack = append(stack, num)
		}
	}
	return stack[0]
}

//Merge Intervals
func merge(intervals [][]int) [][]int {
	if len(intervals) == 0 {
		return [][]int{}
	}

	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})

	res := [][]int{intervals[0]}

	for i := 1; i < len(intervals); i++ {
		current := intervals[i]
		lastAdded := res[len(res)-1]

		if current[0] <= lastAdded[1] {
			if current[1] > lastAdded[1] {
				lastAdded[1] = current[1]
			}
		} else {
			res = append(res, current)
		}
	}

	return res
}
