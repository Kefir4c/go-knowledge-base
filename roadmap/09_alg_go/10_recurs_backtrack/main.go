package main

import "sort"

//SUBSETS II
/*
Given an integer array nums that may contain duplicates, return all possible subsets (the power set).
The solution set must not contain duplicate subsets. Return the solution in any order.

Example 1:
Input: nums = [1,2,2]
Output: [[],[1],[1,2],[1,2,2],[2],[2,2]]

Example 2:
Input: nums = [0]
Output: [[],[0]]
*/
func subsetsWithDup(nums []int) [][]int {
	res := [][]int{}
	path := []int{}
	sort.Ints(nums)

	var bt func(start int)
	bt = func(start int) {
		copyPath := make([]int, len(path))
		copy(copyPath, path)
		res = append(res, copyPath)

		for i := start; i < len(nums); i++ {

			if i > start && nums[i] == nums[i-1] {
				continue
			}

			path = append(path, nums[i])
			bt(i + 1)
			path = path[:len(path)-1]
		}
	}
	bt(0)
	return res
}

//GENERATE PARENTHESES
/*
Given n pairs of parentheses, write a function to generate all combinations of well-formed parentheses.

Example 1:
Input: n = 3
Output: ["((()))","(()())","(())()","()(())","()()()"]

Example 2:
Input: n = 1
Output: ["()"]
*/
func generateParenthesis(n int) []string {
	res := []string{}

	var bt func(curr string, open, close int)
	bt = func(curr string, open, close int) {
		if len(curr) == 2*n {
			res = append(res, curr)
			return
		}
		if open < n {
			bt(curr+"(", open+1, close)
		}
		if close < open {
			bt(curr+")", open, close+1)
		}
	}
	bt("", 0, 0)
	return res
}

//Letter Combinations of a Phone Number
/*
Given a string containing digits from 2-9 inclusive, return all possible letter combinations that the number could represent. Return the answer in any order.
A mapping of digits to letters (just like on the telephone buttons) is given below. Note that 1 does not map to any letters.

Example 1:
Input: digits = "23"
Output: ["ad","ae","af","bd","be","bf","cd","ce","cf"]

Example 2:
Input: digits = "2"
Output: ["a","b","c"]
*/
func letterCombinations(digits string) []string {
	if len(digits) == 0 {
		return []string{}
	}

	phone := map[byte]string{
		'2': "abc", '3': "def", '4': "ghi", '5': "jkl",
		'6': "mno", '7': "pqrs", '8': "tuv", '9': "wxyz",
	}

	res := []string{}
	path := []byte{}

	var bt func(idx int)
	bt = func(idx int) {
		if len(path) == len(digits) {
			res = append(res, string(path))
			return
		}

		digit := digits[idx]
		letters := phone[digit]

		for i := 0; i < len(letters); i++ {
			path = append(path, letters[i])
			bt(idx + 1)
			path = path[:len(path)-1]
		}
	}
	bt(0)
	return res
}

//Word Search
/*
Given an m x n grid of characters board and a string word, return true if word exists in the grid.
The word can be constructed from letters of sequentially adjacent cells, where adjacent cells are horizontally
or vertically neighboring. The same letter cell may not be used more than once.

Example 1:
Input: board = [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word = "ABCCED"
Output: true

Example 2:
Input: board = [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word = "SEE"
Output: true

Example 3:
Input: board = [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word = "ABCB"
Output: false
*/
func exist(board [][]byte, word string) bool {
	rows := len(board)
	cols := len(board[0])

	var bt func(r, c, idx int) bool
	bt = func(r, c, idx int) bool {
		if idx == len(word) {
			return true
		}

		if r < 0 || r >= rows || c < 0 || c >= cols || board[r][c] != word[idx] {
			return false
		}

		temp := board[r][c]
		board[r][c] = '#'

		found := bt(r+1, c, idx+1) ||
			bt(r-1, c, idx+1) ||
			bt(r, c+1, idx+1) ||
			bt(r, c-1, idx+1)

		board[r][c] = temp
		return found
	}

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if bt(r, c, 0) {
				return true
			}
		}
	}
	return false
}

//Subsets
/*
Given an integer array nums of unique elements, return all possible subsets (the power set).
The solution set must not contain duplicate subsets. Return the solution in any order.

Example 1:
Input: nums = [1,2,3]
Output: [[],[1],[2],[1,2],[3],[1,3],[2,3],[1,2,3]]

Example 2:
Input: nums = [0]
Output: [[],[0]]
*/
func subsets(nums []int) [][]int {
	res := [][]int{}
	path := []int{}
	sort.Ints(nums)

	var bt func(start int)
	bt = func(start int) {
		copyPath := make([]int, len(path))
		copy(copyPath, path)
		res = append(res, copyPath)

		for i := start; i < len(nums); i++ {
			path = append(path, nums[i])
			bt(i + 1)
			path = path[:len(path)-1]
		}
	}
	bt(0)
	return res
}

//Permutations
/*
Given an array nums of distinct integers, return all the possible permutations. You can return the answer in any order.

Example 1:
Input: nums = [1,2,3]
Output: [[1,2,3],[1,3,2],[2,1,3],[2,3,1],[3,1,2],[3,2,1]]

Example 2:
Input: nums = [0,1]
Output: [[0,1],[1,0]]

Example 3:
Input: nums = [1]
Output: [[1]]
*/
func permute(nums []int) [][]int {
	res := [][]int{}
	path := []int{}
	visited := make(map[int]bool, len(nums))

	var bt func()
	bt = func() {
		if len(path) == len(nums) {
			copyPath := make([]int, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
			return
		}

		for i := 0; i < len(nums); i++ {
			if visited[i] {
				continue
			}

			visited[i] = true
			path = append(path, nums[i])

			bt()

			path = path[:len(path)-1]
			visited[i] = false
		}
	}

	bt()
	return res
}

//Permutations II
/*
Given a collection of numbers,that might contain duplicates, return all possible unique permutations in any order.nums

Example 1:
Input: nums = [1,1,2]
Output:
[[1,1,2],
 [1,2,1],
 [2,1,1]]

Example 2:
Input: nums = [1,2,3]
Output: [[1,2,3],[1,3,2],[2,1,3],[2,3,1],[3,1,2],[3,2,1]]
*/
func permuteUnique(nums []int) [][]int {
	res := [][]int{}
	path := []int{}
	visited := make(map[int]bool, len(nums))
	sort.Ints(nums)

	var bt func()
	bt = func() {
		if len(path) == len(nums) {
			copyPath := make([]int, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
			return
		}

		for i := 0; i < len(nums); i++ {
			if visited[i] {
				continue
			}

			if i > 0 && nums[i] == nums[i-1] && !visited[i-1] {
				continue
			}

			visited[i] = true
			path = append(path, nums[i])

			bt()

			path = path[:len(path)-1]
			visited[i] = false
		}
	}

	bt()
	return res
}

//Combination Sum
/*
Given an array of distinct integers and a target integer , return a list of all unique combinations of
where the chosen numbers sum to . You may return the combinations in any order.candidatestargetcandidatestarget
The same number may be chosen from an unlimited number of times. Two combinations are unique if the candidatesfrequency of at least one of the chosen numbers is different.
The test cases are generated such that the number of unique combinations that sum up to is less than combinations for the given input.target150

Example 1:
Input: candidates = [2,3,6,7], target = 7
Output: [[2,2,3],[7]]
Explanation:
2 and 3 are candidates, and 2 + 2 + 3 = 7. Note that 2 can be used multiple times.
7 is a candidate, and 7 = 7.
These are the only two combinations.

Example 2:
Input: candidates = [2,3,5], target = 8
Output: [[2,2,2,2],[2,3,3],[3,5]]

Example 3:
Input: candidates = [2], target = 1
Output: []
*/
func combinationSum(candidates []int, target int) [][]int {
	res := [][]int{}
	path := []int{}
	sort.Ints(candidates)

	var bt func(start, remain int)
	bt = func(start, remain int) {
		if remain == 0 {
			copyPath := make([]int, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
			return
		}

		for i := start; i < len(candidates); i++ {
			if candidates[i] > remain {
				break
			}

			path = append(path, candidates[i])
			bt(i, remain-candidates[i])
			path = path[:len(path)-1]
		}
	}

	bt(0, target)
	return res
}

//Combination Sum II
/*
Given a collection of candidate numbers () and a target number (),
find all unique combinations in  where the candidate numbers sum to .candidatestargetcandidatestarget
Each number in  may only be used once in the combination.candidates
Note: The solution set must not contain duplicate combinations.

Example 1:
Input: candidates = [10,1,2,7,6,1,5], target = 8
Output:
[
[1,1,6],
[1,2,5],
[1,7],
[2,6]
]

Example 2:
Input: candidates = [2,5,2,1,2], target = 5
Output:
[
[1,2,2],
[5]
]
*/
func combinationSum2(candidates []int, target int) [][]int {
	res := [][]int{}
	path := []int{}
	sort.Ints(candidates)

	var bt func(start, remain int)
	bt = func(start, remain int) {
		if remain == 0 {
			copyPath := make([]int, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
			return
		}

		for i := start; i < len(candidates); i++ {
			if candidates[i] > remain {
				break
			}

			if i > start && candidates[i] == candidates[i-1] {
				continue
			}
			path = append(path, candidates[i])
			bt(i+1, remain-candidates[i])
			path = path[:len(path)-1]
		}
	}

	bt(0, target)
	return res
}

//Palindrome Partitioning
/*
Given a string , partition such that every sssubstring of the partition is a palindrome. Return all possible palindrome partitioning of .s

Example 1:
Input: s = "aab"
Output: [["a","a","b"],["aa","b"]]

Example 2:
Input: s = "a"
Output: [["a"]]
*/
func partition(s string) [][]string {
	res := [][]string{}
	path := []string{}

	var bt func(start int)
	bt = func(start int) {
		if start == len(s) {
			copyPath := make([]string, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
			return
		}

		for i := start; i < len(s); i++ {
			sub := s[start : i+1]

			if palidrome(sub) {
				path = append(path, sub)
				bt(i + 1)
				path = path[:len(path)-1]
			}
		}
	}
	bt(0)
	return res
}

func palidrome(str string) bool {
	left, right := 0, len(str)-1
	for left < right {
		if str[left] != str[right] {
			return false
		}
		left++
		right--
	}
	return true
}
