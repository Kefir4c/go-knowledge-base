package main

import "math"

//Median of Two Sorted Arrays
/*
Given two sorted arrays and of size and respectively, return the median of the two sorted arrays.nums1nums2mn
The overall run time complexity should be .O(log (m+n))

Example 1:
Input: nums1 = [1,3], nums2 = [2]
Output: 2.00000
Explanation: merged array = [1,2,3] and median is 2.

Example 2:
Input: nums1 = [1,2], nums2 = [3,4]
Output: 2.50000
Explanation: merged array = [1,2,3,4] and median is (2 + 3) / 2 = 2.5.
*/
func findMedianSortedArrays(nums1 []int, nums2 []int) float64 {
	if len(nums1) > len(nums2) {
		return findMedianSortedArrays(nums2, nums1)
	}

	m, n := len(nums1), len(nums2)
	low, high := 0, m

	for low <= high {
		partitionX := low + (high-low)/2
		partitionY := (m+n+1)/2 - partitionX

		maxLeftX := math.MinInt32
		if partitionX != 0 {
			maxLeftX = nums1[partitionX-1]
		}
		minRightX := math.MaxInt32
		if partitionX != m {
			minRightX = nums1[partitionX]
		}

		maxLeftY := math.MinInt32
		if partitionY != 0 {
			maxLeftY = nums2[partitionY-1]
		}
		minRightY := math.MaxInt32
		if partitionY != n {
			minRightY = nums2[partitionY]
		}

		if maxLeftX <= minRightY && maxLeftY <= minRightX {
			if (m+n)%2 == 0 {
				return float64(max(maxLeftX, maxLeftY)+min(minRightX, minRightY)) / 2.0
			}
			return float64(max(maxLeftX, maxLeftY))
		} else if maxLeftX > minRightY {
			// Too far right in nums1
			high = partitionX - 1
		} else {
			low = partitionX + 1
		}
	}
	return 0.0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//Search in Rotated Sorted Array
/*
There is an integer array nums sorted in ascending order (with distinct values).
Prior to being passed to your function, nums is possibly left rotated at an unknown
index k (1 <= k < nums.length) such that the resulting array is [nums[k], nums[k+1], ..., nums[n-1], nums[0], nums[1], ..., nums[k-1]] (0-indexed).
For example, [0,1,2,4,5,6,7] might be left rotated by 3 indices and become [4,5,6,7,0,1,2].
Given the array nums after the possible rotation and an integer target, return the index of target if it is in nums, or -1 if it is not in nums.
You must write an algorithm with O(log n) runtime complexity.

Example 1:
Input: nums = [4,5,6,7,0,1,2], target = 0
Output: 4

Example 2:
Input: nums = [4,5,6,7,0,1,2], target = 3
Output: -1

Example 3:
Input: nums = [1], target = 0
Output: -1
*/
func search(nums []int, target int) int {
	left, right := 0, len(nums)-1

	for left <= right {
		mid := left + (right-left)/2

		if nums[mid] == target {
			return mid
		}

		if nums[left] <= nums[mid] {
			if nums[left] <= target && target < nums[mid] {
				right = mid - 1
			} else {
				left = mid + 1
			}
		} else {
			if nums[mid] < target && target <= nums[right] {
				left = mid + 1
			} else {
				right = mid - 1
			}
		}
	}

	return -1
}

//Find First and Last Position of Element in Sorted Array
/*
Given an array of integers nums sorted in non-decreasing order,
find the starting and ending position of a given target value.
If target is not found in the array, return [-1, -1].
You must write an algorithm with O(log n) runtime complexity.

Example 1:
Input: nums = [5,7,7,8,8,10], target = 8
Output: [3,4]

Example 2:
Input: nums = [5,7,7,8,8,10], target = 6
Output: [-1,-1]

Example 3:
Input: nums = [], target = 0
Output: [-1,-1]
*/
func searchRange(nums []int, target int) []int {
	res := []int{-1, -1}
	lo, hi := 0, len(nums)-1

	for lo <= hi {
		mid := lo + (hi-lo)/2

		if nums[mid] == target {
			res[0] = mid
			hi = mid - 1
		} else if nums[mid] < target {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	if res[0] == -1 {
		return res
	}

	lo, hi = 0, len(nums)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2

		if nums[mid] == target {
			res[1] = mid
			lo = mid + 1
		} else if nums[mid] < target {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return res
}

//Search a 2D Matrix
/*
You are given an integer matrix with the following two properties:m x nmatrix
Each row is sorted in non-decreasing order.
The first integer of each row is greater than the last integer of the previous row.
Given an integer , return if is in or otherwise.targettruetargetmatrixfalse
You must write a solution in time complexity.O(log(m * n))

Example 1:
Input: matrix = [[1,3,5,7],[10,11,16,20],[23,30,34,60]], target = 3
Output: true

Example 2:
Input: matrix = [[1,3,5,7],[10,11,16,20],[23,30,34,60]], target = 13
Output: false
*/
func searchMatrix(matrix [][]int, target int) bool {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return false
	}

	m := len(matrix)
	n := len(matrix[0])

	lo, hi := 0, n*m-1

	for lo <= hi {
		mid := lo + (hi-lo)/2

		row := mid / n
		col := mid % n
		val := matrix[row][col]

		if val == target {
			return true
		} else if val < target {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return false
}

//Search a 2D Matrix II
/*
Write an efficient algorithm that searches for a value in an integer matrix.
This matrix has the following properties:targetm x nmatrix
Integers in each row are sorted in ascending from left to right.
Integers in each column are sorted in ascending from top to bottom.

Example 1:
Input: matrix = [[1,4,7,11,15],[2,5,8,12,19],[3,6,9,16,22],[10,13,14,17,24],[18,21,23,26,30]], target = 5
Output: true

Example 2:
Input: matrix = [[1,4,7,11,15],[2,5,8,12,19],[3,6,9,16,22],[10,13,14,17,24],[18,21,23,26,30]], target = 20
Output: false
*/
func searchMatrix(matrix [][]int, target int) bool {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return false
	}

	m := len(matrix)
	n := len(matrix[0])
	row := 0
	col := n - 1

	for row < m && col >= 0 {
		current := matrix[row][col]

		if current == target {
			return true
		} else if current < target {
			row++
		} else {
			col--
		}
	}
	return false
}
