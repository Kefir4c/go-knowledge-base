package main

import (
	"math"
	"strings"
)

//Longest Palindromic Substring
/*
Given a string s, return the longest palindromic substring in s.

Example 1:
Input: s = "babad"
Output: "bab"
Explanation: "aba" is also a valid answer.

Example 2:
Input: s = "cbbd"
Output: "bb"
*/
func longestPalindrome(s string) string {
	runes := []rune(s)
	if len(runes) < 2 {
		return s
	}

	start, maxLen := 0, 1

	for i := 0; i < len(runes); i++ {
		expandAroundCenter(runes, i, i, &start, &maxLen)
		expandAroundCenter(runes, i, i+1, &start, &maxLen)
	}
	return string(runes[start : start+maxLen])
}

func expandAroundCenter(runes []rune, left, right int, start, maxLen *int) {
	for left >= 0 && right < len(runes) && runes[left] == runes[right] {
		if right-left+1 > *maxLen {
			*start = left
			*maxLen = right - left + 1
		}
		left--
		right++
	}
}

//String to Integer (atoi)
/*
Implement the myAtoi(string s) function, which converts a string to a 32-bit signed integer.
The algorithm for myAtoi(string s) is as follows:
Whitespace: Ignore any leading whitespace (" ").
Signedness: Determine the sign by checking if the next character is '-' or '+', assuming positivity if neither present.
Conversion: Read the integer by skipping leading zeros until a non-digit character is encountered
or the end of the string is reached. If no digits were read, then the result is 0.
Rounding: If the integer is out of the 32-bit signed integer range [-231, 231 - 1], then round the integer to remain
in the range. Specifically, integers less than -231 should be rounded to -231, and integers greater than 231 - 1 should be rounded to 231 - 1.
Return the integer as the final result.
Note: You must not use any built-in library function that converts a string to a number
(for example, atoi, stoi, Integer.parseInt, int(), parseInt, Number()). Perform the conversion manually.

Example 1:
Input:
s := "42"
Output: 42
Explanation:
The underlined characters are what is read in and the caret is the current reader position.
Step 1: "42" (no characters read because there is no leading whitespace)
         ^
Step 2: "42" (no characters read because there is neither a '-' nor '+')
         ^
Step 3: "42" ("42" is read in)

Example 2:
Input:
s := " -042"
Output: -42
Explanation:
Step 1: "   -042" (leading whitespace is read and ignored)
            ^
Step 2: "   -042" ('-' is read, so the result should be negative)
             ^
Step 3: "   -042" ("042" is read in, leading zeros ignored in the result)
*/
func myAtoi(s string) int {
	i := 0
	n := len(s)

	for i < n && s[i] == ' ' {
		i++
	}
	if i == n {
		return 0
	}

	sign := 1
	if s[i] == '+' {
		i++
	} else if s[i] == '-' {
		sign = -1
		i++
	}

	num := 0
	for i < n && s[i] >= '0' && s[i] <= '9' {
		digit := int(s[i] - '0')

		if num > math.MaxInt32/10 || (num == math.MaxInt32/10 && digit > 7) {
			if sign == 1 {
				return math.MaxInt32
			}
			return math.MinInt32
		}
		num = num*10 + digit
		i++
	}
	return num * sign
}

//Decode String
/*
Given an encoded string, return its decoded string.
The encoding rule is: k[encoded_string], where the encoded_string inside the square brackets
is being repeated exactly k times. Note that k is guaranteed to be a positive integer.
You may assume that the input string is always valid; there are no extra white spaces, square
brackets are well-formed, etc. Furthermore, you may assume that the original data does not
contain any digits and that digits are only for those repeat numbers, k. For example, there will not be input like 3a or 2[4].
The test cases are generated so that the length of the output will never exceed 105.

Example 1:
Input:
s := "3[a]2[bc]"
Output: "aaabcbc"

Example 2:
Input:
s := "3[a2[c]]"
Output: "accaccacc"

Example 3:
Input:
s := "2[abc]3[cd]ef"
Output: "abcabccdcdcdef"
*/
func decodeString(s string) string {
	countStack := []int{}
	stringStack := []string{}

	currNum := 0
	currStr := ""

	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			currNum = currNum*10 + int(ch-'0')

		} else if ch == '[' {
			countStack = append(countStack, currNum)
			stringStack = append(stringStack, currStr)
			currNum = 0
			currStr = ""

		} else if ch == ']' {
			count := countStack[len(countStack)-1]
			countStack = countStack[:len(countStack)-1]

			prevStr := stringStack[len(stringStack)-1]
			stringStack = stringStack[:len(stringStack)-1]

			currStr = prevStr + strings.Repeat(currStr, count)

		} else {
			currStr += string(ch)
		}
	}

	return currStr
}

//Reverse Words in a String
/*
Given an input string s, reverse the order of the words.
A word is defined as a sequence of non-space characters. The words in s will be separated by at least one space.
Return a string of the words in reverse order concatenated by a single space.
Note that s may contain leading or trailing spaces or multiple spaces between two words.
The returned string should only have a single space separating the words. Do not include any extra spaces.

Example 1:
Input:
s := "the sky is blue"
Output: "blue is sky the"

Example 2:
Input:
s := "  hello world  "
Output: "world hello"
Explanation: Your reversed string should not contain leading or trailing spaces.

Example 3:
Input:
s := "a good   example"
Output: "example good a"
Explanation: You need to reduce multiple spaces between two words to a single space in the reversed string.
*/
func reverseWords(s string) string {
	var sb strings.Builder
	i := len(s) - 1

	for i >= 0 {
		for i >= 0 && s[i] == ' ' {
			i--
		}
		if i < 0 {
			break
		}

		j := i
		for j >= 0 && s[j] != ' ' {
			j--
		}

		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(s[j+1 : i+1])

		i = j
	}
	return sb.String()
}
