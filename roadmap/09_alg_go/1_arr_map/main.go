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
