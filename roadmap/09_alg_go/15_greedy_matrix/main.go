package main

//Spiral Matrix
/*
Given an m x n matrix, return all elements of the matrix in spiral order.

Example 1:
Input:
matrix := [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
Output: [1,2,3,6,9,8,7,4,5]

Example 2:
Input:
matrix := [][]int{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}}
Output: [1,2,3,4,8,12,11,10,9,5,6,7]
*/
func spiralOrder(matrix [][]int) []int {
	if len(matrix) == 0 {
		return []int{}
	}

	top, bottom := 0, len(matrix)-1
	left, right := 0, len(matrix[0])-1
	result := []int{}

	for top <= bottom && left <= right {
		for i := left; i <= right; i++ {
			result = append(result, matrix[top][i])
		}
		top++

		for i := top; i <= bottom; i++ {
			result = append(result, matrix[i][right])
		}
		right--

		if top <= bottom {
			for i := right; i >= left; i-- {
				result = append(result, matrix[bottom][i])
			}
			bottom--
		}

		if left <= right {
			for i := bottom; i >= top; i-- {
				result = append(result, matrix[i][left])
			}
			left++
		}
	}
	return result
}

//Maximum Subarray
/*
Given an integer array nums, find the subarray with the largest sum, and return its sum.

Example 1:
Input:
nums := []int{-2, 1, -3, 4, -1, 2, 1, -5, 4}
Output: 6
Explanation: The subarray [4,-1,2,1] has the largest sum 6.

Example 2:
Input:
nums := []int{1}
Output: 1
Explanation: The subarray [1] has the largest sum 1.

Example 3:
Input:
nums := []int{5, 4, -1, 7, 8}
Output: 23
Explanation: The subarray [5,4,-1,7,8] has the largest sum 23.
*/
func maxSubArray(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	maxSum := nums[0]
	currSum := 0

	for _, num := range nums {
		currSum += num

		if currSum > maxSum {
			maxSum = currSum
		}
		if currSum < 0 {
			currSum = 0
		}
	}
	return maxSum
}

//Jump Game
/*
You are given an integer array nums. You are initially positioned at the array's first index,
and each element in the array represents your maximum jump length at that position.
Return true if you can reach the last index, or false otherwise.

Example 1:
Input:
nums := []int{2, 3, 1, 1, 4}
Output: true
Explanation: Jump 1 step from index 0 to 1, then 3 steps to the last index.

Example 2:
Input:
nums := []int{3, 2, 1, 0, 4}
Output: false
Explanation: You will always arrive at index 3 no matter what. Its maximum jump length is 0, which makes it impossible to reach the last index.
*/
func canJump(nums []int) bool {
	maxReach := 0

	for i := 0; i < len(nums); i++ {
		if i > maxReach {
			return false
		}

		if i+nums[i] > maxReach {
			maxReach = i + nums[i]
		}

		if maxReach >= len(nums)-1 {
			return true
		}
	}
	return true
}
func canJump(nums []int) bool {
	goal := len(nums) - 1

	for i := len(nums) - 2; i >= 0; i-- {
		if i+nums[i] >= goal {
			goal = i
		}
	}
	return goal == 0
}

//Rotate Image
/*
You are given an 2D representing an image, rotate the image by 90 degrees (clockwise).n x nmatrix
You have to rotate the image in-place, which means you have to modify the input 2D matrix directly. DO NOT allocate another 2D matrix and do the rotation.

Example 1:
Input:
matrix := [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
Output: [[7,4,1],[8,5,2],[9,6,3]]

Example 2:
Input:
matrix := [][]int{{5, 1, 9, 11}, {2, 4, 8, 10}, {13, 3, 6, 7}, {15, 14, 12, 16}}
Output: [[15,13,2,5],[14,3,4,1],[12,6,8,9],[16,7,10,11]]
*/
func rotate(matrix [][]int) {
	n := len(matrix)

	for i := 0; i < (n+1)/2; i++ {
		for j := 0; j < n/2; j++ {
			temp := matrix[i][j]
			matrix[i][j] = matrix[n-1-j][i]
			matrix[n-1-j][i] = matrix[n-1-i][n-1-j]
			matrix[n-1-i][n-1-j] = matrix[j][n-1-i]
			matrix[j][n-1-i] = temp
		}
	}
}

//Set Matrix Zeroes
/*
Given an m x n integer matrix matrix, if an element is 0, set its entire row and column to 0's.
You must do it in place.

Example 1:
Input:
matrix := [][]int{{1, 1, 1}, {1, 0, 1}, {1, 1, 1}}
Output: [[1,0,1],[0,0,0],[1,0,1]]

Example 2:
Input:
matrix := [][]int{{0, 1, 2, 0}, {3, 4, 5, 2}, {1, 3, 1, 5}}
Output: [[0,0,0,0],[0,4,5,0],[0,3,1,0]]
*/
func setZeroes(matrix [][]int) {
	if len(matrix) == 0 {
		return
	}
	rows, cols := len(matrix), len(matrix[0])

	firstRowHasZero := false
	firstColHasZero := false

	for c := 0; c < cols; c++ {
		if matrix[0][c] == 0 {
			firstRowHasZero = true
			break
		}
	}

	for r := 0; r < rows; r++ {
		if matrix[r][0] == 0 {
			firstColHasZero = true
			break
		}
	}

	for r := 1; r < rows; r++ {
		for c := 1; c < cols; c++ {
			if matrix[r][c] == 0 {
				matrix[r][0] = 0
				matrix[0][c] = 0
			}
		}
	}

	for r := 1; r < rows; r++ {
		for c := 1; c < cols; c++ {
			if matrix[r][0] == 0 || matrix[0][c] == 0 {
				matrix[r][c] = 0
			}
		}
	}

	if firstRowHasZero {
		for c := 0; c < cols; c++ {
			matrix[0][c] = 0
		}
	}

	if firstColHasZero {
		for r := 0; r < rows; r++ {
			matrix[r][0] = 0
		}
	}
}
