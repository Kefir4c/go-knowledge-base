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
