package main

import (
	"container/heap"
	"sort"
	"strings"
)

//Kth Largest Element in an Array
/*
Given an integer array nums and an integer k, return the kth largest element in the array.
Note that it is the kth largest element in the sorted order, not the kth distinct element.
Can you solve it without sorting?

Example 1:
Input: nums = [3,2,1,5,6,4], k = 2
Output: 5

Example 2:
Input: nums = [3,2,3,1,2,4,5,5,6], k = 4
Output: 4

*/
type IntMinHeap []int

func (h IntMinHeap) Len() int           { return len(h) }
func (h IntMinHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *IntMinHeap) Push(x interface{}) {
	*h = append(*h, x.(int))
}

func (h *IntMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func findKthLargest(nums []int, k int) int {
	h := &IntMinHeap{}
	heap.Init(h)

	for _, num := range nums {
		heap.Push(h, num)

		if h.Len() > k {
			heap.Pop(h)
		}
	}
	return (*h)[0]
}

//Top K Frequent Elements
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
type Element struct {
	num  int
	freq int
}

type MinHeap []Element

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].freq < h[j].freq }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x any) {
	*h = append(*h, x.(Element))
}

func (h *MinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func topKFrequent(nums []int, k int) []int {
	freqMap := make(map[int]int)
	for _, num := range nums {
		freqMap[num]++
	}

	h := &MinHeap{}
	heap.Init(h)

	for num, freq := range freqMap {
		heap.Push(h, Element{num, freq})

		if h.Len() > k {
			heap.Pop(h)
		}
	}

	result := make([]int, 0, k)
	for h.Len() > 0 {
		el := heap.Pop(h).(Element)
		result = append(result, el.num)
	}
	return result
}

//K Closest Points to Origin
/*
Given an array of points where points[i] = [xi, yi] represents a point on the X-Y plane and an integer k, return the k closest points to the origin (0, 0).
The distance between two points on the X-Y plane is the Euclidean distance (i.e., √(x1 - x2)2 + (y1 - y2)2).
You may return the answer in any order. The answer is guaranteed to be unique (except for the order that it is in).

Example 1:
Input: points = [[1,3],[-2,2]], k = 1
Output: [[-2,2]]
Explanation:
The distance between (1, 3) and the origin is sqrt(10).
The distance between (-2, 2) and the origin is sqrt(8).
Since sqrt(8) < sqrt(10), (-2, 2) is closer to the origin.
We only want the closest k = 1 points from the origin, so the answer is just [[-2,2]].

Example 2:
Input: points = [[3,3],[5,-1],[-2,4]], k = 2
Output: [[3,3],[-2,4]]
Explanation: The answer [[-2,4],[3,3]] would also be accepted.
*/
type Points struct {
	x, y int
	dist int
}

type MaxHeap []Points

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i].dist > h[j].dist }
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x any) {
	*h = append(*h, x.(Points))
}

func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func kClosest(points [][]int, k int) [][]int {
	h := &MaxHeap{}
	heap.Init(h)

	for _, p := range points {
		dist := p[0]*p[0] + p[1]*p[1]
		heap.Push(h, Points{p[0], p[1], dist})

		if h.Len() > k {
			heap.Pop(h)
		}
	}

	res := make([][]int, 0, k)
	for h.Len() > 0 {
		p := heap.Pop(h).(Points)
		res = append(res, []int{p.x, p.y})
	}
	return res
}

//Merge k Sorted Lists
/*
You are given an array of k linked-lists lists, each linked-list is sorted in ascending order.
Merge all the linked-lists into one sorted linked-list and return it.

Example 1:
Input: lists = [[1,4,5],[1,3,4],[2,6]]
Output: [1,1,2,3,4,4,5,6]
Explanation: The linked-lists are:
[
  1->4->5,
  1->3->4,
  2->6
]
merging them into one sorted linked list:
1->1->2->3->4->4->5->6

Example 2:
Input: lists = []
Output: []

Example 3:
Input: lists = [[]]
Output: []
*/
/**
 * Definition for singly-linked list.
 * type ListNode struct {
 *     Val int
 *     Next *ListNode
 * }
 */

type minHeap []*ListNode

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].Val < h[j].Val }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *minHeap) Push(x any) {
	*h = append(*h, x.(*ListNode))
}

func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func mergeKLists(lists []*ListNode) *ListNode {
	h := &minHeap{}
	heap.Init(h)

	for _, list := range lists {
		if list != nil {
			heap.Push(h, list)
		}
	}

	dummy := &ListNode{}
	tail := dummy

	for h.Len() > 0 {
		minNode := heap.Pop(h).(*ListNode)

		tail.Next = minNode
		tail = tail.Next

		if minNode.Next != nil {
			heap.Push(h, minNode.Next)
		}
	}
	return dummy.Next
}

//Find Median from Data Stream
/*
The median is the middle value in an ordered integer list. If the size of the list is even,
there is no middle value, and the median is the mean of the two middle values.
For example, for arr = [2,3,4], the median is 3.
For example, for arr = [2,3], the median is (2 + 3) / 2 = 2.5.
Implement the MedianFinder class:
MedianFinder() initializes the MedianFinder object.
void addNum(int num) adds the integer num from the data stream to the data structure.
double findMedian() returns the median of all elements so far.
Answers within 10-5 of the actual answer will be accepted.

Example 1:
Input
["MedianFinder", "addNum", "addNum", "findMedian", "addNum", "findMedian"]
[[], [1], [2], [], [3], []]
Output
[null, null, null, 1.5, null, 2.0]
Explanation
MedianFinder medianFinder = new MedianFinder();
medianFinder.addNum(1);    // arr = [1]
medianFinder.addNum(2);    // arr = [1, 2]
medianFinder.findMedian(); // return 1.5 (i.e., (1 + 2) / 2)
medianFinder.addNum(3);    // arr[1, 2, 3]
medianFinder.findMedian(); // return 2.0
*/
type MaxHeap struct{ sort.IntSlice }

func (h MaxHeap) Less(i, j int) bool { return h.IntSlice[i] > h.IntSlice[j] }

func (h *MaxHeap) Push(x any) {
	h.IntSlice = append(h.IntSlice, x.(int))
}
func (h *MaxHeap) Pop() any {
	old := h.IntSlice
	n := len(old)
	x := old[n-1]
	h.IntSlice = old[:n-1]
	return x
}

type MinHeap struct{ sort.IntSlice }

func (h *MinHeap) Push(x any) {
	h.IntSlice = append(h.IntSlice, x.(int))
}
func (h *MinHeap) Pop() any {
	old := h.IntSlice
	n := len(old)
	x := old[n-1]
	h.IntSlice = old[:n-1]
	return x
}

type MedianFinder struct {
	small *MaxHeap
	large *MinHeap
}

func Constructor() MedianFinder {
	small := &MaxHeap{}
	large := &MinHeap{}
	heap.Init(small)
	heap.Init(large)
	return MedianFinder{small: small, large: large}
}

func (this *MedianFinder) AddNum(num int) {
	heap.Push(this.small, num)

	if this.small.Len() > 0 && this.large.Len() > 0 && this.small.IntSlice[0] > this.large.IntSlice[0] {
		val := heap.Pop(this.small).(int)
		heap.Push(this.large, val)
	}

	if this.small.Len() > this.large.Len()+1 {
		val := heap.Pop(this.small).(int)
		heap.Push(this.large, val)
	} else if this.large.Len() > this.small.Len() {
		val := heap.Pop(this.large).(int)
		heap.Push(this.small, val)
	}
}

func (this *MedianFinder) FindMedian() float64 {
	if this.small.Len() == this.large.Len() {
		return (float64(this.small.IntSlice[0]) + float64(this.large.IntSlice[0])) / 2.0
	}

	return float64(this.small.IntSlice[0])
}

/**
 * Your MedianFinder object will be instantiated and called as such:
 * obj := Constructor();
 * obj.AddNum(num);
 * param_2 := obj.FindMedian();
 */

//Task Scheduler
/*
You are given an array of CPU , each labeled with a letter from A to Z, and a number . Each CPU interval can
be idle or allow the completion of one task. Tasks can be completed in any order, but there's a constraint:
there has to be a gap of at least intervals between two tasks with the same label.tasksnn
Return the minimum number of CPU intervals required to complete all tasks.

Example 1:
Input: tasks = ["A","A","A","B","B","B"], n = 2
Output: 8
Explanation: A possible sequence is: A -> B -> idle -> A -> B -> idle -> A -> B.
After completing task A, you must wait two intervals before doing A again. The same applies to task B. In the
3rd interval, neither A nor B can be done, so you idle. By the 4th interval, you can do A again as 2 intervals have passed.

Example 2:
Input: tasks = ["A","C","A","B","D","B"], n = 1
Output: 6
Explanation: A possible sequence is: A -> B -> C -> D -> A -> B.
With a cooling interval of 1, you can repeat a task after just one other task.

Example 3:
Input: tasks = ["A","A","A", "B","B","B"], n = 3
Output: 10
Explanation: A possible sequence is: A -> B -> idle -> idle -> A -> B -> idle -> idle -> A -> B.
There are only two types of tasks, A and B, which need to be separated by 3 intervals. This leads to idling twice between repetitions of these tasks.
*/
type MaxHeapInt []int

func (h MaxHeapInt) Len() int           { return len(h) }
func (h MaxHeapInt) Less(i, j int) bool { return h[i] > h[j] }
func (h MaxHeapInt) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeapInt) Push(x any) { *h = append(*h, x.(int)) }
func (h *MaxHeapInt) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func leastInterval(tasks []byte, n int) int {
	counts := make(map[byte]int)
	for _, task := range tasks {
		counts[task]++
	}

	h := &MaxHeapInt{}
	for _, count := range counts {
		heap.Push(h, count)
	}

	time := 0
	queue := [][2]int{}

	for h.Len() > 0 || len(queue) > 0 {
		time++

		if h.Len() > 0 {
			count := heap.Pop(h).(int) - 1
			if count > 0 {
				queue = append(queue, [2]int{count, time + n})
			}
		}

		if len(queue) > 0 && queue[0][1] == time {
			heap.Push(h, queue[0][0])
			queue = queue[1:]
		}
	}
	return time
}

//Reorganize String
/*
Given a string s, rearrange the characters of s so that any two adjacent characters are not the same.
Return any possible rearrangement of s or return "" if not possible.

Example 1:
Input: s = "aab"
Output: "aba"

Example 2:
Input: s = "aaab"
Output: ""
*/
type Item struct {
	char byte
	freq int
}

type MaxHeap []Item

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i].freq > h[j].freq }
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x any) { *h = append(*h, x.(Item)) }
func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func reorganizeString(s string) string {
	counts := make(map[byte]int)
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}

	h := &MaxHeap{}
	heap.Init(h)
	for char, freq := range counts {
		heap.Push(h, Item{char, freq})
	}

	var res strings.Builder

	for h.Len() >= 2 {
		first := heap.Pop(h).(Item)
		second := heap.Pop(h).(Item)

		res.WriteByte(first.char)
		res.WriteByte(second.char)

		first.freq--
		second.freq--

		if first.freq > 0 {
			heap.Push(h, first)
		}
		if second.freq > 0 {
			heap.Push(h, second)
		}
	}

	if h.Len() > 0 {
		last := heap.Pop(h).(Item)
		if last.freq > 1 {
			return ""
		}
		res.WriteByte(last.char)
	}

	return res.String()
}
