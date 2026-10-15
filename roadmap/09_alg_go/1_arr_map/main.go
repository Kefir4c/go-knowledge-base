package main

import "sort"

// Two Sum
/*
Вам даётся массив целых чисел и целое число, которые возвращают индексы двух чисел так, что они складываются в цель.numstarget
Можно предположить, что каждый вход будет иметь ровно одно решение, и не использовать один и тот же элемент дважды.
Вы можете вернуть ответ в любом порядке.

Пример 1:
Input: nums = [2,7,11,15], target = 9
Output: [0,1]
Explanation: Because nums[0] + nums[1] == 9, we return [0, 1].

Пример 2:
Input: nums = [3,2,4], target = 6
Output: [1,2]

Пример 3:
Input: nums = [3,3], target = 6
Output: [0,1]
*/

func twoSum(nums []int, target int) []int {
	lookup := make(map[int]int)
	for idx, num := range nums {
		diff := target - num
		foundIdx, found := lookup[diff]
		if found {
			return []int{idx, foundIdx}
		}
		lookup[num] = idx
	}
	return []int{-1, -1}
}

// Contains Duplicate
/*
Given an integer array , return if any value appears at least twice in the array, and return if every element is distinct.numstruefalse

Example 1:
Input: nums = [1,2,3,1]
Output: true
Explanation:
The element 1 occurs at the indices 0 and 3.

Example 2:
Input: nums = [1,2,3,4]
Output: false
Explanation:
All elements are distinct.

Example 3:
Input: nums = [1,1,1,3,3,4,3,2,4,2]
Output: true
*/

func containsDuplicate(nums []int) bool {
	seen := make(map[int]bool)
	for _, num := range nums {
		if seen[num] {
			return true
		}
		seen[num] = true
	}
	return false
}

// Valid Anagram
/*
Given two strings s and t, return true if t is an anagram of s, and false otherwise.

Example 1:
Input: s = "anagram", t = "nagaram"
Output: true

Example 2:
Input: s = "rat", t = "car"
Output: false

Constraints:
1 <= s.length, t.length <= 5 * 104
s and t consist of lowercase English letters.

Follow up: What if the inputs contain Unicode characters? How would you adapt your solution to such a case?
*/
func isAnagram(s string, t string) bool {
	if len(s) != len(t) {
		return false
	}
	freq := make(map[rune]int)

	for _, r := range s {
		freq[r]++
	}
	for _, r := range t {
		freq[r]--
		if freq[r] < 0 {
			return false
		}
	}
	return true
}

//Group Anagrams
/*
Given an array of strings strs, group the anagrams together. You can return the answer in any order.

Example 1:
Input: strs = ["eat","tea","tan","ate","nat","bat"]
Output: [["bat"],["nat","tan"],["ate","eat","tea"]]

Explanation:
There is no string in strs that can be rearranged to form "bat".
The strings "nat" and "tan" are anagrams as they can be rearranged to form each other.
The strings "ate", "eat", and "tea" are anagrams as they can be rearranged to form each other.
Example 2:

Input: strs = [""]
Output: [[""]]
Example 3:

Input: strs = ["a"]
Output: [["a"]]
*/
func groupAnagrams(strs []string) [][]string {
	groups := make(map[string][]string)
	for _, s := range strs {
		key := sortString(s)
		groups[key] = append(groups[key], s)
	}
	result := make([][]string, 0, len(groups))
	for _, g := range groups {
		result = append(result, g)
	}
	return result
}

func sortString(s string) string {
	b := []byte(s)
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	return string(b)
}

// Top K Frequent Elements
/*
Given an integer array nums and an integer k, return the k most frequent elements. You may return the answer in any order.

Example 1:
Input: nums = [1,1,1,2,2,3], k = 2
Output: [1,2]

Example 2:
Input: nums = [1], k = 1
Output: [1]

Example 3:
Input: nums = [1,2,1,2,1,2,3,1,3,2], k = 2
Output: [1,2]
*/
func topKFrequent(nums []int, k int) []int {
	freq := make(map[int]int)
	for _, n := range nums {
		freq[n]++
	}

	pairs := make([][2]int, 0, len(freq))
	for v, f := range freq {
		pairs = append(pairs, [2]int{v, f})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i][1] > pairs[j][1] })

	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = pairs[i][0]
	}
	return result
}

//Product of Array Except Self
/*
Given an integer array nums, return an array answer such that answer[i]
is equal to the product of all the elements of nums except nums[i].
The product of any prefix or suffix of nums is guaranteed to fit in a 32-bit integer.
You must write an algorithm that runs in O(n) time and without using the division operation.

Example 1:
Input: nums = [1,2,3,4]
Output: [24,12,8,6]

Example 2:
Input: nums = [-1,1,0,-3,3]
Output: [0,0,9,0,0]

*/
func productExceptSelf(nums []int) []int {
	n := len(nums)
	result := make([]int, n)
	result[0] = 1
	for i := 1; i < n; i++ {
		result[i] = result[i-1] * nums[i-1]
	}

	right := 1
	for i := n - 1; i >= 0; i-- {
		result[i] *= right
		right *= nums[i]
	}
	return result
}

//Subarray Sum Equals K
/*
Given an array of integers and an integer , return the total
number of subarrays whose sum equals to .nums k k
A subarray is a contiguous non-empty sequence of elements within an array.

Example 1:
Input: nums = [1,1,1], k = 2
Output: 2

Example 2:
Input: nums = [1,2,3], k = 3
Output: 2
*/
func subarraySum(nums []int, k int) int {
	count := 0
	prefix := 0
	seen := map[int]int{0: 1}

	for _, n := range nums {
		prefix += n
		count += seen[prefix-k]
		seen[prefix]++
	}
	return count
}

//Longest Consecutive Sequence
/*
Given an unsorted array of integers nums, return the length of
the longest consecutive elements sequence.
You must write an algorithm that runs in O(n) time.

Example 1:
Input: nums = [100,4,200,1,3,2]
Output: 4
Explanation: The longest consecutive elements sequence is [1, 2, 3, 4]. Therefore its length is 4.

Example 2:
Input: nums = [0,3,7,2,5,8,4,6,0,1]
Output: 9

Example 3:
Input: nums = [1,0,1,2]
Output: 3
*/
func longestConsecutive(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	set := make(map[int]bool)
	for _, num := range nums {
		set[num] = true
	}

	longest := 0

	for num := range set {
		if !set[num-1] {
			length := 1
			for set[num+length] {
				length++
			}
			if length > longest {
				longest = length
			}
		}
	}
	return longest
}

// Majority Element
/*
Given an array nums of size n, return the majority element.
The majority element is the element that appears more than ⌊n / 2⌋ times.
You may assume that the majority element always exists in the array.

Example 1:
Input: nums = [3,2,3]
Output: 3

Example 2:
Input: nums = [2,2,1,1,1,2,2]
Output: 2
*/
func majorityElement(nums []int) int {
	count := make(map[int]int)
	for _, n := range nums {
		count[n]++
	}

	for num, freq := range count {
		if freq > len(nums)/2 {
			return num
		}
	}
	return 0
}

// Majority Element II
/*
Given an integer array of size n, find all elements that appear more than ⌊n / 3⌋ times.

Example 1:
Input: nums = [3,2,3]
Output: [3]

Example 2:
Input: nums = [1]
Output: [1]

Example 3:
Input: nums = [1,2]
Output: [1,2]
*/
func majorityElement(nums []int) []int {
	res := make([]int, 0, len(nums))
	count := make(map[int]int)
	for _, n := range nums {
		count[n]++
	}

	for num, freq := range count {
		if freq > len(nums)/3 {
			res = append(res, num)
		}
	}
	return res
}

//3Sum
/*
Given an integer array nums, return all the triplets [nums[i], nums[j], nums[k]] such
that i != j, i != k, and j != k, and nums[i] + nums[j] + nums[k] == 0.
Notice that the solution set must not contain duplicate triplets.

Example 1:
Input: nums = [-1,0,1,2,-1,-4]
Output: [[-1,-1,2],[-1,0,1]]
Explanation:
nums[0] + nums[1] + nums[2] = (-1) + 0 + 1 = 0.
nums[1] + nums[2] + nums[4] = 0 + 1 + (-1) = 0.
nums[0] + nums[3] + nums[4] = (-1) + 2 + (-1) = 0.
The distinct triplets are [-1,0,1] and [-1,-1,2].
Notice that the order of the output and the order of the triplets does not matter.

Example 2:
Input: nums = [0,1,1]
Output: []
Explanation: The only possible triplet does not sum up to 0.

Example 3:
Input: nums = [0,0,0]
Output: [[0,0,0]]
Explanation: The only possible triplet sums up to 0.
*/
func threeSum(nums []int) [][]int {
	n := len(nums)
	sort.Ints(nums)
	result := [][]int{}

	for i := 0; i < n-2; i++ {
		if i > 0 && nums[i] == nums[i-1] {
			continue
		}
		if nums[i] > 0 {
			break
		}

		left := i + 1
		right := n - 1

		for left < right {
			sum := nums[i] + nums[left] + nums[right]

			if sum == 0 {
				result = append(result, []int{nums[i], nums[left], nums[right]})

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
	return result
}

//Find All Anagrams in a String
/*
Given two strings s and p, return an array of all the start indices of p's anagrams in s. You may return the answer in any order.

Example 1:
Input: s = "cbaebabacd", p = "abc"
Output: [0,6]
Explanation:
The substring with start index = 0 is "cba", which is an anagram of "abc".
The substring with start index = 6 is "bac", which is an anagram of "abc".

Example 2:
Input: s = "abab", p = "ab"
Output: [0,1,2]
Explanation:
The substring with start index = 0 is "ab", which is an anagram of "ab".
The substring with start index = 1 is "ba", which is an anagram of "ab".
The substring with start index = 2 is "ab", which is an anagram of "ab".
*/
func findAnagrams(s string, p string) []int {
	result := []int{}
	pl, sl := len(p), len(s)

	if sl < pl {
		return result
	}

	var pFreq, sFreq [26]int
	for i := 0; i < pl; i++ {
		pFreq[p[i]-'a']++
		sFreq[s[i]-'a']++
	}

	if pFreq == sFreq {
		result = append(result, 0)
	}

	for right := pl; right < sl; right++ {
		left := right - pl
		sFreq[s[left]-'a']--
		sFreq[s[right]-'a']++

		if pFreq == sFreq {
			result = append(result, left+1)
		}
	}
	return result
}

//Continuous Subarray Sum
/*
Given an integer array nums and an integer k,
return true if nums has a good subarray or false otherwise.
A good subarray is a subarray where:
its length is at least two, and
the sum of the elements of the subarray is a multiple of k.

Note that:
A subarray is a contiguous part of the array.
An integer x is a multiple of k if there exists an integer n such that x = n * k. 0 is always a multiple of k.

Example 1:
Input: nums = [23,2,4,6,7], k = 6
Output: true
Explanation: [2, 4] is a continuous subarray of size 2 whose elements sum up to 6.

Example 2:
Input: nums = [23,2,6,4,7], k = 6
Output: true
Explanation: [23, 2, 6, 4, 7] is an continuous
subarray of size 5 whose elements sum up to 42.
42 is a multiple of 6 because 42 = 7 * 6 and 7 is an integer.

Example 3:
Input: nums = [23,2,6,4,7], k = 13
Output: false
*/
func checkSubarraySum(nums []int, k int) bool {
	seen := map[int]int{0: -1} // ← -1, не 1
	prefix := 0

	for i, num := range nums {
		prefix += num
		rem := prefix % k

		if prevIdx, ok := seen[rem]; ok {
			if i-prevIdx >= 2 {
				return true
			}
		} else {
			seen[rem] = i
		}
	}
	return false
}

//Contiguous Array
/*
Given a binary array nums, return the maximum length of a
contiguous subarray with an equal number of 0 and 1.

Example 1:
Input: nums = [0,1]
Output: 2
Explanation: [0, 1] is the longest contiguous subarray with an equal number of 0 and 1.

Example 2:
Input: nums = [0,1,0]
Output: 2
Explanation: [0, 1] (or [1, 0]) is a longest contiguous subarray with equal number of 0 and 1.

Example 3:
Input: nums = [0,1,1,1,1,1,0,0,0]
Output: 6
Explanation: [1,1,1,0,0,0] is the longest contiguous subarray with equal number of 0 and 1.
*/
func findMaxLength(nums []int) int {
	seen := map[int]int{0: -1} // prefix → первый индекс
	prefix := 0
	maxLen := 0

	for i, num := range nums {
		if num == 0 {
			prefix--
		} else {
			prefix++
		}

		if prevIdx, ok := seen[prefix]; ok {
			length := i - prevIdx
			if length > maxLen {
				maxLen = length
			}
		} else {
			seen[prefix] = i // ← ТОЛЬКО первый индекс
		}
	}

	return maxLen
}

//Boats to Save People
/*
You are given an array people where people[i] is the weight of the ith person,
and an infinite number of boats where each boat can carry a maximum weight of limit.
Each boat carries at most two people at the same time, provided the sum of the
 weight of those people is at most limit.
Return the minimum number of boats to carry every given person.

Example 1:
Input: people = [1,2], limit = 3
Output: 1
Explanation: 1 boat (1, 2)

Example 2:
Input: people = [3,2,2,1], limit = 3
Output: 3
Explanation: 3 boats (1, 2), (2) and (3)

Example 3:
Input: people = [3,5,3,4], limit = 5
Output: 4
Explanation: 4 boats (3), (3), (4), (5)
*/
func numRescueBoats(people []int, limit int) int {
	sort.Ints(people)
	left := 0
	right := len(people) - 1
	boat := 0

	if left <= right {
		if people[left]+people[right] <= limit {
			left++
		}
		right--
		boat++
	}
	return boat
}

//Longest Substring Without Repeating Characters
/*
Given a string , find the length of the longests substring without duplicate characters.

Example 1:
Input: s = "abcabcbb"
Output: 3
Explanation: The answer is "abc", with the length of 3. Note that "bca" and "cab" are also correct answers.

Example 2:
Input: s = "bbbbb"
Output: 1
Explanation: The answer is "b", with the length of 1.

Example 3:
Input: s = "pwwkew"
Output: 3
Explanation: The answer is "wke", with the length of 3.
Notice that the answer must be a substring, "pwke" is a subsequence and not a substring.
*/
func lengthOfLongestSubstring(s string) int {
	seen := make(map[byte]bool)
	left := 0
	maxLen := 0

	for right := 0; right < len(s); right++ {
		for seen[s[right]] {
			delete(seen, s[left])
			left++
		}
		seen[s[right]] = true

		if right-left+1 > maxLen {
			maxLen = right - left + 1
		}
	}
	return maxLen
}
