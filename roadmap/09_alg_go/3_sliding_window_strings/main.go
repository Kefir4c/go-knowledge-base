package main

//Repeated DNA Sequences
/*
The DNA sequence is composed of a series of nucleotides abbreviated as , , , and .'A''C''G''T'
For example, is a DNA sequence."ACGAATTCCG"
When studying DNA, it is useful to identify repeated sequences within the DNA.
Given a string that represents a DNA sequence, return all the 10-letter-long sequences
(substrings) that occur more than once in a DNA molecule. You may return the answer in any order.s

Example 1:
Input: s = "AAAAACCCCCAAAAACCCCCCAAAAAGGGTTT"
Output: ["AAAAACCCCC","CCCCCAAAAA"]

Example 2:
Input: s = "AAAAAAAAAAAAA"
Output: ["AAAAAAAAAA"]
*/
func findRepeatedDnaSequences(s string) []string {
	if len(s) < 10 {
		return []string{}
	}

	seen := make(map[string]int)

	for i := 0; i <= len(s)-10; i++ {
		sub := s[i : i+10]
		seen[sub]++
	}

	result := []string{}
	for sub, count := range seen {
		if count > 1 {
			result = append(result, sub)
		}
	}
	return result
}

//Longest Substring with At Least K Repeating Characters
/*
Given a string and an integer , return the length of the longest substring of
such that the frequency of each character in this substring is greater than or equal to .sksk
if no such substring exists, return 0.

Example 1:
Input: s = "aaabb", k = 3
Output: 3
Explanation: The longest substring is "aaa", as 'a' is repeated 3 times.

Example 2:
Input: s = "ababbc", k = 2
Output: 5
Explanation: The longest substring is "ababb", as 'a' is repeated 2 times and 'b' is repeated 3 times.
*/
func longestSubstring(s string, k int) int {
	if len(s) < k {
		return 0
	}

	// Считаем частоты
	freq := make(map[byte]int)
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}

	// Ищем разделитель
	for i := 0; i < len(s); i++ {
		if freq[s[i]] < k {
			// Рекурсивно вызываем САМУ СЕБЯ
			left := longestSubstring(s[:i], k)
			right := longestSubstring(s[i+1:], k)

			if left > right {
				return left
			}
			return right
		}
	}

	return len(s)
}

//Arithmetic Slices
/*
An integer array is called arithmetic if it consists of at least three elements
and if the difference between any two consecutive elements is the same.
For example, [1,3,5,7,9], [7,7,7,7], and [3,-1,-5,-9] are arithmetic sequences.
Given an integer array nums, return the number of arithmetic subarrays of nums.
A subarray is a contiguous subsequence of the array.

Example 1:
Input: nums = [1,2,3,4]
Output: 3
Explanation: We have 3 arithmetic slices in nums: [1, 2, 3], [2, 3, 4] and [1,2,3,4] itself.

Example 2:
Input: nums = [1]
Output: 0
*/
func numberOfArithmeticSlices(nums []int) int {
	n := len(nums)
	if n < 3 {
		return 0
	}
	count := 0
	cur := 0

	for i := 2; i < n; i++ {
		if nums[i]-nums[i-1] == nums[i-1]-nums[i-2] {
			cur++
			count += cur
		} else {
			cur = 0
		}
	}
	return count
}

//Permutation in String
/*
Given two strings s1 and s2, return true if s2 contains a permutation of s1, or false otherwise.
In other words, return true if one of s1's permutations is the substring of s2.

Example 1:
Input: s1 = "ab", s2 = "eidbaooo"
Output: true
Explanation: s2 contains one permutation of s1 ("ba").

Example 2:
Input: s1 = "ab", s2 = "eidboaoo"
Output: false
*/
func checkInclusion(s1 string, s2 string) bool {
	if len(s2) < len(s1) {
		return false
	}
	var s1Freq [26]int
	var windowFreq [26]int

	for i := 0; i < len(s1); i++ {
		s1Freq[s1[i]-'a']++
		windowFreq[s2[i]-'a']++
	}

	if s1Freq == windowFreq {
		return true
	}

	for rig := len(s1); rig < len(s2); rig++ {
		left := rig - len(s1)

		windowFreq[s2[left]-'a']--
		windowFreq[s2[rig]-'a']++

		if s1Freq == windowFreq {
			return true
		}
	}
	return false
}

//Find K Closest Elements
/*
Given a sorted integer array arr, two integers k and x, return the k closest
integers to x in the array. The result should also be sorted in ascending order.
An integer a is closer to x than an integer b if:

|a - x| < |b - x|, or
|a - x| == |b - x| and a < b

Example 1:
Input: arr = [1,2,3,4,5], k = 4, x = 3
Output: [1,2,3,4]

Example 2:
Input: arr = [1,1,2,3,4,5], k = 4, x = -1
Output: [1,1,2,3]
*/
func findClosestElements(arr []int, k int, x int) []int {
	if len(arr) < k {
		return []int{0}
	}
	lo := 0
	hi := len(arr) - k

	for lo < hi {
		mid := (lo + hi) / 2

		if x-arr[mid] > arr[mid+k]-x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return arr[lo : lo+k]
}

//Maximum Length of Repeated Subarray
/*
Given two integer arrays and , return the maximum length of a subarray that appears in both arrays.nums1nums2

Example 1:
Input: nums1 = [1,2,3,2,1], nums2 = [3,2,1,4,7]
Output: 3
Explanation: The repeated subarray with maximum length is [3,2,1].

Example 2:
Input: nums1 = [0,0,0,0,0], nums2 = [0,0,0,0,0]
Output: 5
Explanation: The repeated subarray with maximum length is [0,0,0,0,0].
*/
func findLength(nums1 []int, nums2 []int) int {
	if len(nums1) < len(nums2) {
		nums1, nums2 = nums2, nums1
	}

	n, m := len(nums1), len(nums2)

	dp := make([]int, m+1)
	maxLen := 0

	for i := 1; i <= n; i++ {
		prev := 0

		for j := 1; j <= m; j++ {
			temp := dp[j]

			if nums1[i-1] == nums2[j-1] {
				dp[j] = prev + 1
				if dp[j] > maxLen {
					maxLen = dp[j]
				}
			} else {
				dp[j] = 0
			}
			prev = temp
		}
	}
	return maxLen
}
