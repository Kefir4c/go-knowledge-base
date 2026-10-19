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

//Koko Eating Bananas
/*
Koko loves to eat bananas. There are n piles of bananas, the ith pile
has piles[i] bananas. The guards have gone and will come back in h hours.
Koko can decide her bananas-per-hour eating speed of k. Each hour,
she chooses some pile of bananas and eats k bananas from that pile.
If the pile has less than k bananas, she eats all of them instead
and will not eat any more bananas during this hour.
Koko likes to eat slowly but still wants to finish eating all
the bananas before the guards return.
Return the minimum integer k such that she can eat all the bananas within h hours.

Example 1:
Input: piles = [3,6,7,11], h = 8
Output: 4

Example 2:
Input: piles = [30,11,23,4,20], h = 5
Output: 30

Example 3:
Input: piles = [30,11,23,4,20], h = 6
Output: 23
*/
func minEatingSpeed(piles []int, h int) int {
	maxPile := 0
	for _, p := range piles {
		if p > maxPile {
			maxPile = p
		}
	}

	low, high := 1, maxPile
	ans := high

	for low <= high {
		mid := low + (high-low)/2

		totalHours := 0
		for _, p := range piles {
			totalHours += (p + mid - 1) / mid
		}

		if totalHours <= h {
			ans = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	return ans
}

//Capacity To Ship Packages Within D Days
/*
A conveyor belt has packages that must be shipped from one port to another within days.days
The package on the conveyor belt has a weight of .
Each day, we load the ship with packages on the conveyor belt (in the order given by ).
We may not load more weight than the maximum weight capacity of the
ship.ithweights[i]weights
Return the least weight capacity of the ship that will result in all
the packages on the conveyor belt being shipped within days.days

Example 1:
Input: weights = [1,2,3,4,5,6,7,8,9,10], days = 5
Output: 15
Explanation: A ship capacity of 15 is the minimum to ship all the packages in 5 days like this:
1st day: 1, 2, 3, 4, 5
2nd day: 6, 7
3rd day: 8
4th day: 9
5th day: 10
Note that the cargo must be shipped in the order given, so using
a ship of capacity 14 and splitting the packages into
parts like (2, 3, 4, 5), (1, 6, 7), (8), (9), (10) is not allowed.

Example 2:
Input: weights = [3,2,2,4,1,4], days = 3
Output: 6
Explanation: A ship capacity of 6 is the minimum to ship
all the packages in 3 days like this:
1st day: 3, 2
2nd day: 2, 4
3rd day: 1, 4

Example 3:
Input: weights = [1,2,3,1,1], days = 4
Output: 3
Explanation:
1st day: 1
2nd day: 2
3rd day: 3
4th day: 1, 1
*/
func shipWithinDays(weights []int, days int) int {
	lo, hi := 0, 0
	for _, w := range weights {
		if w > lo {
			lo = w
		}
		hi += w
	}

	ans := hi

	for lo <= hi {
		mid := lo + (hi-lo)/2
		totalDays := 1
		currentLoad := 0

		for _, w := range weights {
			if currentLoad+w > mid {
				totalDays++
				currentLoad = 0
			}
			currentLoad += w
		}

		if totalDays <= days {
			ans = mid
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return ans
}

//Find Minimum in Rotated Sorted Array
/*
Suppose an array of length sorted in ascending order is rotated
between and times. For example, the array might become:n1nnums = [0,1,2,4,5,6,7]
[4,5,6,7,0,1,2] if it was rotated times.4
[0,1,2,4,5,6,7] if it was rotated times.7
Notice that rotating an array 1 time results in the array .
[a[0], a[1], a[2], ..., a[n-1]][a[n-1], a[0], a[1], a[2], ..., a[n-2]]
Given the sorted rotated array of unique elements, return
the minimum element of this array.nums
You must write an algorithm that runs in .O(log n) time

Example 1:
Input: nums = [3,4,5,1,2]
Output: 1
Explanation: The original array was [1,2,3,4,5] rotated 3 times.

Example 2:
Input: nums = [4,5,6,7,0,1,2]
Output: 0
Explanation: The original array was [0,1,2,4,5,6,7] and it was rotated 4 times.

Example 3:
Input: nums = [11,13,15,17]
Output: 11
Explanation: The original array was [11,13,15,17] and it was rotated 4 times.
*/
func findMin(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	lef, rig := 0, len(nums)-1

	for lef < rig {
		mid := lef + (rig-lef)/2

		if nums[mid] > nums[rig] {
			lef = mid + 1
		} else {
			rig = mid
		}
	}
	return nums[lef]
}

//Find the Duplicate Number
/*
Given an array of integers nums containing n + 1 integers
where each integer is in the range [1, n] inclusive.
There is only one repeated number in nums, return this repeated number.
You must solve the problem without modifying the
array nums and using only constant extra space.

Example 1:
Input: nums = [1,3,4,2,2]
Output: 2

Example 2:
Input: nums = [3,1,3,4,2]
Output: 3

Example 3:
Input: nums = [3,3,3,3,3]
Output: 3
*/
func findDuplicate(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	lo, hi := 1, len(nums)-1

	for lo < hi {
		mid := lo + (hi-lo)/2
		count := 0

		for _, n := range nums {
			if n <= mid {
				count++
			}
		}
		if count > mid {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}
