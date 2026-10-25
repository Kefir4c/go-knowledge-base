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
		level := []int{}

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
