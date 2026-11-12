package _9_alg_go

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
