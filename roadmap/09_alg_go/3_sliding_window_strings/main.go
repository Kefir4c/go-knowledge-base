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
