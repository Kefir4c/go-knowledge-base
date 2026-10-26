package main

//Path Sum II
/*
Given the root of a binary tree and an integer targetSum, return all root-to-leaf
paths where the sum of the node values in the path equals targetSum. Each path
should be returned as a list of the node values, not node references.
A root-to-leaf path is a path starting from the root and ending at any leaf node.
A leaf is a node with no children.

Example 1:

Input: root = [5,4,8,11,null,13,4,7,2,null,null,5,1], targetSum = 22
Output: [[5,4,11,2],[5,8,4,5]]
Explanation: There are two paths whose sum equals targetSum:
5 + 4 + 11 + 2 = 22
5 + 8 + 4 + 5 = 22

Example 2:
Input: root = [1,2,3], targetSum = 5
Output: []

Example 3:
Input: root = [1,2], targetSum = 0
Output: []
*/
func pathSum(root *TreeNode, targetSum int) [][]int {
	res := [][]int{}
	path := []int{}

	var dfs func(node *TreeNode, remaining int)
	dfs = func(node *TreeNode, remaining int) {
		if node == nil {
			return
		}

		path = append(path, node.Val)
		remaining -= node.Val

		if node.Left == nil && node.Right == nil && remaining == 0 {
			copyPath := make([]int, len(path))
			copy(copyPath, path)
			res = append(res, copyPath)
		} else {
			dfs(node.Left, remaining)
			dfs(node.Right, remaining)
		}
		path = path[:len(path)-1]
	}
	dfs(root, targetSum)
	return res
}

//Binary Tree Inorder Traversal
/*
Given the of a binary tree, return the inorder traversal of its nodes' values.root

Example 1:
Input: root = [1,null,2,3]
Output: [1,3,2]

Example 2:
Input: root = [1,2,3,4,5,null,8,null,null,6,7,9]
Output: [4,2,6,5,7,1,3,9,8]

Example 3:
Input: root = []
Output: []

Example 4:
Input: root = [1]
Output: [1]
*/
func inorderTraversal(root *TreeNode) []int {
	res := []int{}

	var dfs func(node *TreeNode)
	dfs = func(node *TreeNode) {
		if node == nil {
			return
		}
		dfs(node.Left)
		res = append(res, node.Val)
		dfs(node.Right)
	}
	dfs(root)
	return res
}

func inorderTraversal(root *TreeNode) []int {
	res := []int{}
	stack := []*TreeNode{}
	curr := root

	for curr != nil || len(stack) > 0 {
		for curr != nil {
			stack = append(stack, curr)
			curr = curr.Left
		}

		curr = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		res = append(res, curr.Val)

		curr = curr.Right
	}
	return res
}

//Binary Tree Preorder Traversal
/*
Given the root of a binary tree, return the preorder traversal of its nodes' values.

Example 1:
Input: root = [1,null,2,3]
Output: [1,2,3]

Example 2:
Input: root = [1,2,3,4,5,null,8,null,null,6,7,9]
Output: [1,2,4,5,6,7,3,8,9]


Example 3:
Input: root = []
Output: []


Example 4:
Input: root = [1]
Output: [1]
*/
func preorderTraversal(root *TreeNode) []int {
	res := []int{}

	var dfs func(node *TreeNode)
	dfs = func(node *TreeNode) {
		if node == nil {
			return
		}
		res = append(res, node.Val)
		dfs(node.Left)
		dfs(node.Right)
	}
	dfs(root)
	return res
}
func preorderTraversal(root *TreeNode) []int {
	if root == nil {
		return []int{}
	}

	res := []int{}
	stack := []*TreeNode{root}

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		res = append(res, node.Val)

		if node.Right != nil {
			stack = append(stack, node.Right)
		}
		if node.Left != nil {
			stack = append(stack, node.Left)
		}
	}
	return res
}

//Binary Tree Postorder Traversal
/*
Given the root of a binary tree, return the postorder traversal of its nodes' values.

Example 1:
Input: root = [1,null,2,3]
Output: [3,2,1]

Example 2:
Input: root = [1,2,3,4,5,null,8,null,null,6,7,9]
Output: [4,6,7,5,2,9,8,3,1]

Example 3:
Input: root = []
Output: []

Example 4:
Input: root = [1]
Output: [1]
*/
func postorderTraversal(root *TreeNode) []int {
	res := []int{}

	var dfs func(node *TreeNode)
	dfs = func(node *TreeNode) {
		if node == nil {
			return
		}
		dfs(node.Left)
		dfs(node.Right)
		res = append(res, node.Val)
	}
	dfs(root)
	return res
}
func postorderTraversal(root *TreeNode) []int {
	res := []int{}
	stack := []*TreeNode{}
	curr := root
	var lastVisited *TreeNode

	for curr != nil || len(stack) > 0 {
		for curr != nil {
			stack = append(stack, curr)
			curr = curr.Left
		}

		peek := stack[len(stack)-1]

		if peek.Right != nil && peek.Right != lastVisited {
			curr = peek.Right
		} else {
			res = append(res, peek.Val)
			stack = stack[:len(stack)-1]
			lastVisited = peek
		}
	}
	return res
}

//Binary Tree Level Order Traversal
/*
Given the root of a binary tree, return the level order traversal
of its nodes' values. (i.e., from left to right, level by level).

Example 1:
Input: root = [3,9,20,null,null,15,7]
Output: [[3],[9,20],[15,7]]

Example 2:
Input: root = [1]
Output: [[1]]

Example 3:
Input: root = []
Output: []
*/
func levelOrder(root *TreeNode) [][]int {
	res := [][]int{}
	if root == nil {
		return res
	}
	queue := []*TreeNode{root}

	for len(queue) > 0 {
		levelSize := len(queue)
		level := make([]int, levelSize)

		for i := 0; i < levelSize; i++ {
			node := queue[0]
			queue = queue[1:]
			level = append(level, node.Val)

			if node.Left != nil {
				queue = append(queue, node.Left)
			}
			if node.Right != nil {
				queue = append(queue, node.Right)
			}
		}
		res = append(res, level)
	}
	return res
}

//Lowest Common Ancestor of a Binary Tree
/*
Given a binary tree, find the lowest common ancestor (LCA) of two given nodes in the tree.
According to the definition of LCA on Wikipedia: “The lowest common ancestor is defined between two nodes p and q
as the lowest node in T that has both p and q as descendants (where we allow a node to be a descendant of itself).”

Example 1:
Input: root = [3,5,1,6,2,0,8,null,null,7,4], p = 5, q = 1
Output: 3
Explanation: The LCA of nodes 5 and 1 is 3.

Example 2:
Input: root = [3,5,1,6,2,0,8,null,null,7,4], p = 5, q = 4
Output: 5
Explanation: The LCA of nodes 5 and 4 is 5, since a node can be a descendant of itself according to the LCA definition.

Example 3:
Input: root = [1,2], p = 1, q = 2
Output: 1
*/
func lowestCommonAncestor(root, p, q *TreeNode) *TreeNode {
	if root == nil || root == p || root == q {
		return root
	}

	left := lowestCommonAncestor(root.Left, p, q)
	right := lowestCommonAncestor(root.Right, p, q)

	if left != nil && right != nil {
		return root
	}

	if left != nil {
		return left
	}
	return right
}
func lowestCommonAncestor(root, p, q *TreeNode) *TreeNode {
	parent := make(map[*TreeNode]*TreeNode)
	parent[root] = nil
	stack := []*TreeNode{root}

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node.Left != nil {
			parent[node.Left] = node
			stack = append(stack, node.Left)
		}
		if node.Right != nil {
			parent[node.Right] = node
			stack = append(stack, node.Right)
		}
	}

	ancestors := make(map[*TreeNode]bool)
	for p != nil {
		ancestors[p] = true
		p = parent[p]
	}

	for q != nil {
		if ancestors[q] {
			return q
		}
		q = parent[q]
	}
	return nil
}

//Validate Binary Search Tree
/*
Given the root of a binary tree, determine if it is a valid binary search tree (BST).
A valid BST is defined as follows:
The left subtree of a node contains only nodes with keys strictly less than the node's key.
The right subtree of a node contains only nodes with keys strictly greater than the node's key.
Both the left and right subtrees must also be binary search trees.

Example 1:
Input: root = [2,1,3]
Output: true

Example 2:
Input: root = [5,1,4,null,null,3,6]
Output: false
Explanation: The root node's value is 5 but its right child's value is 4.
*/
func isValidBST(root *TreeNode) bool {
	var validate func(node *TreeNode, min, max *int) bool
	validate = func(node *TreeNode, min, max *int) bool {
		if node == nil {
			return true
		}

		if (min != nil && node.Val <= *min) || (max != nil && node.Val >= *max) {
			return false
		}

		return validate(node.Left, min, &node.Val) && validate(node.Right, &node.Val, max)
	}
	return validate(root, nil, nil)
}

//Path Sum III
/*
Given the root of a binary tree and an integer targetSum, return the number of paths where the sum of the values along the path equals targetSum.
The path does not need to start or end at the root or a leaf, but it must go downwards (i.e., traveling only from parent nodes to child nodes).

Example 1:
Input: root = [10,5,-3,3,2,null,11,3,-2,null,1], targetSum = 8
Output: 3
Explanation: The paths that sum to 8 are shown.

Example 2:
Input: root = [5,4,8,11,null,13,4,7,2,null,null,5,1], targetSum = 22
Output: 3
*/
func pathSum(root *TreeNode, targetSum int) int {
	prefixSum := map[int]int{0: 1}
	count := 0
	var dfs func(node *TreeNode, currSum int)
	dfs = func(node *TreeNode, currSum int) {
		if node == nil {
			return
		}

		currSum += node.Val

		if freq, ok := prefixSum[currSum-targetSum]; ok {
			count += freq
		}

		prefixSum[currSum]++

		dfs(node.Left, currSum)
		dfs(node.Right, currSum)

		prefixSum[currSum]--
	}
	dfs(root, 0)
	return count
}

//Kth Smallest Element in a BST
/*
Given the of a binary search tree, and an integer , return the smallest
value (1-indexed) of all the values of the nodes in the tree.rootkkth

Example 1:
Input: root = [3,1,4,null,2], k = 1
Output: 1

Example 2:
Input: root = [5,3,6,2,4,null,null,1], k = 3
Output: 3
*/
func kthSmallest(root *TreeNode, k int) int {
	stack := []*TreeNode{}
	curr := root

	for curr != nil || len(stack) > 0 {
		for curr != nil {
			stack = append(stack, curr)
			curr = curr.Left
		}

		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		k--
		if k == 0 {
			return curr.Val
		}

		curr = curr.Right
	}
	return -1
}

//Binary Tree Zigzag Level Order Traversal
/*
Given the root of a binary tree, return the zigzag level order traversal of its nodes' values.
(i.e., from left to right, then right to left for the next level and alternate between).

Example 1:
Input: root = [3,9,20,null,null,15,7]
Output: [[3],[20,9],[15,7]]

Example 2:
Input: root = [1]
Output: [[1]]

Example 3:
Input: root = []
Output: []
*/
func zigzagLevelOrder(root *TreeNode) [][]int {
	res := [][]int{}
	if root == nil {
		return res
	}
	queue := []*TreeNode{root}
	leftToRight := true

	for len(queue) > 0 {
		levelSize := len(queue)
		level := make([]int, levelSize)

		for i := 0; i < levelSize; i++ {
			node := queue[0]
			queue = queue[1:]

			if leftToRight {
				level[i] = node.Val
			} else {
				level[levelSize-1-i] = node.Val
			}

			if node.Left != nil {
				queue = append(queue, node.Left)
			}
			if node.Right != nil {
				queue = append(queue, node.Right)
			}
		}
		res = append(res, level)
		leftToRight = !leftToRight
	}
	return res
}
