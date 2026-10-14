package main

//Container With Most Water
/*
You are given an integer array height of length n.
There are n vertical lines drawn such that the two endpoints of the ith line are (i, 0) and (i, height[i]).
Find two lines that together with the x-axis form a container, such that the container contains the most water.
Return the maximum amount of water a container can store.
Notice that you may not slant the container.
Example 1:
Input: height = [1,8,6,2,5,4,8,3,7]
Output: 49
Explanation: The above vertical lines are represented by array [1,8,6,2,5,4,8,3,7].
In this case, the max area of water (blue section) the container can contain is 49.

Example 2:
Input: height = [1,1]
Output: 1
*/
func maxArea(height []int) int {
	maxArea := 0
	left, right := 0, len(height)-1

	for left < right {
		min := height[left]
		if height[right] < min {
			min = height[right]
		}

		width := right - left
		area := min * width

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

//Two Sum II - Input Array Is Sorted
/*
You are given a 1-indexed array of integers that is already sorted in non-decreasing order.numbers
Find two numbers such that they add up to a specific number. Let these two numbers be and where .
targetnumbers[index1]numbers[index2]1 <= index1 < index2 <= numbers.length
Return the indices of the two numbers and as an integer array of length 2.index1index2[index1, index2]
The tests are generated such that there is exactly one solution. You may not use the same element twice.
Your solution must use only constant extra space.

Example 1:
Input: numbers = [2,7,11,15], target = 9
Output: [1,2]
Explanation: The sum of 2 and 7 is 9. Therefore, index1 = 1, index2 = 2. We return [1, 2].

Example 2:
Input: numbers = [2,3,4], target = 6
Output: [1,3]
Explanation: The sum of 2 and 4 is 6. Therefore index1 = 1, index2 = 3. We return [1, 3].

Example 3:
Input: numbers = [-1,0], target = -1
Output: [1,2]
Explanation: The sum of -1 and 0 is -1. Therefore index1 = 1, index2 = 2. We return [1, 2].
*/
func twoSum(numbers []int, target int) []int {
	left, right := 0, len(numbers)-1
	for left < right {
		sum := numbers[left] + numbers[right]

		if sum == target {
			return []int{left + 1, right + 1}
		} else if sum < target {
			left++
		} else {
			right--
		}
	}
	return []int{-1, -1}
}

//Sort Colors
/*
You are given an array nums with n objects colored red, white, or blue, sort them in-place so that objects
of the same color are adjacent, with the colors in the order red, white, and blue.
We will use the integers 0, 1, and 2 to represent the color red, white, and blue, respectively.
You must solve this problem without using the library's sort function.

Example 1:
Input: nums = [2,0,2,1,1,0]
Output: [0,0,1,1,2,2]
Explanation:
The array has two 0s, two 1s, and two 2s. Sorting them in-place places all 0s first, then all 1s, then all 2s.

Example 2:
Input: nums = [2,0,1]
Output: [0,1,2]
Explanation:
The array has one each of 0, 1, and 2, arranged in-place in the order 0, 1, 2.
*/
func sortColors(nums []int) {
	low, mid := 0, 0
	hight := len(nums) - 1

	for mid <= hight {
		switch nums[mid] {
		case 0:
			nums[low], nums[mid] = nums[mid], nums[low]
			low++
			mid++
		case 1:
			mid++
		case 2:
			nums[mid], nums[hight] = nums[hight], nums[mid]
			hight--
		}

	}
}

//Longest Palindromic Substring
/*
Given a string , return the longests palindromic substring in .s

Example 1:
Input: s = "babad"
Output: "bab"
Explanation: "aba" is also a valid answer.

Example 2:
Input: s = "cbbd"
Output: "bb"
*/
func longestPalindrome(s string) string {
	if len(s) < 2 {
		return s
	}
	start := 0
	maxLen := 1

	for i := 0; i < len(s); i++ {
		expandAroundCenter(s, i, i, &start, &maxLen)
		expandAroundCenter(s, i, i+1, &start, &maxLen)
	}
	return s[start : start+maxLen]
}

func expandAroundCenter(s string, left, right int, start, maxLen *int) {
	for left >= 0 && right < len(s) && s[left] == s[right] {
		if right-left+1 > *maxLen {
			*start = left
			*maxLen = right - left + 1
		}
		left--
		right++
	}
}

//Trapping Rain Water
/*
Given n non-negative integers representing an elevation map where the width of each bar is 1, compute how much water it can trap after raining.

Example 1:
Input: height = [0,1,0,2,1,0,1,3,2,1,2,1]
Output: 6
Explanation: The above elevation map (black section) is represented by array [0,1,0,2,1,0,1,3,2,1,2,1].
In this case, 6 units of rain water (blue section) are being trapped.

Example 2:
Input: height = [4,2,0,3,2,5]
Output: 9
*/
func trap(height []int) int {
	if len(height) < 3 {
		return 0
	}

	left := 0
	right := len(height) - 1
	maxLeft := 0
	maxRight := 0
	water := 0

	for left < right {
		if height[left] < height[right] {
			if height[left] >= maxLeft {
				maxLeft = height[left]
			} else {
				water += maxLeft - height[left]
			}
			left++
		} else {
			if height[right] >= maxRight {
				maxRight = height[right]
			} else {
				water += maxRight - height[right]
			}
			right--
		}
	}
	return water
}

//Subarray Product Less Than K
/*
You are given an array of integers nums and an integer k.
Return the number of contiguous subarrays where the product
of all the elements in the subarray is strictly less than k.

Example 1:
Input: nums = [10,5,2,6], k = 100
Output: 8
Explanation: The 8 subarrays that have product less than 100 are:
[10], [5], [2], [6], [10, 5], [5, 2], [2, 6], [5, 2, 6]
Note that [10, 5, 2] is not included as the product of 100 is not strictly less than k.

Example 2:
Input: nums = [1,2,3], k = 0
Output: 0
*/
func numSubarrayProductLessThanK(nums []int, k int) int {
	if k < 1 {
		return 0
	}

	product := 1
	left, count := 0, 0

	for right := 0; right < len(nums); right++ {
		product *= nums[right]

		for product >= k {
			product /= nums[left]
			left++
		}
		count += right - left + 1
	}
	return count
}
