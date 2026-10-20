package main

import "sort"

// 3Sum
func threeSum(nums []int) [][]int {
	n := len(nums)
	result := [][]int{}
	sort.Ints(nums)

	for i := 0; i < n-2; i++ {
		if i > 0 && nums[i] == nums[i-1] {
			continue
		}
		if nums[i] > 0 {
			break
		}

		left := i + 1
		rig := n - 1

		for left < rig {
			sum := nums[i] + nums[left] + nums[rig]

			if sum == 0 {
				result = append(result, []int{nums[i], nums[left], nums[rig]})

				for left < rig && nums[left] == nums[left+1] {
					left++
				}
				for left < rig && nums[rig] == nums[rig-1] {
					rig--
				}

				left++
				rig--
			} else if sum < 0 {
				left++
			} else {
				rig--
			}

		}
	}
	return result
}

// Longest Substring Without Repeating Characters
func lengthOfLongestSubstring(s string) int {
	seen := make(map[byte]bool)
	maxLen := 0
	left := 0

	for rig := 0; rig < len(s); rig++ {
		for seen[s[rig]] {
			delete(seen, s[left])
			left++
		}
		seen[s[rig]] = true

		if rig-left+1 > maxLen {
			maxLen = rig - left + 1
		}
	}
	return maxLen
}

// Valid Palindrome
func isPalindrome(s string) bool {
	left, right := 0, len(s)-1

	for left < right {
		for left < right && !isValid(s[left]) {
			left++
		}
		for left < right && !isValid(s[right]) {
			right--
		}

		if toLower(s[left]) != toLower(s[right]) {
			return false
		}

		left++
		right--
	}
	return true
}

func isValid(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
