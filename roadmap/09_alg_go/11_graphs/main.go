package main

//Number of Islands
/*
Given an m x n 2D binary grid grid which represents a map of '1's (land) and '0's (water), return the number of islands.
An island is surrounded by water and is formed by connecting adjacent lands horizontally or vertically.
You may assume all four edges of the grid are all surrounded by water.

Example 1:
Input: grid = [
  ["1","1","1","1","0"],
  ["1","1","0","1","0"],
  ["1","1","0","0","0"],
  ["0","0","0","0","0"]
]
Output: 1

Example 2:
Input: grid = [
  ["1","1","0","0","0"],
  ["1","1","0","0","0"],
  ["0","0","1","0","0"],
  ["0","0","0","1","1"]
]
Output: 3
*/
func numIslands(grid [][]byte) int {
	if len(grid) == 0 {
		return 0
	}

	rows := len(grid)
	cols := len(grid[0])
	count := 0

	var dfs func(r, c int)
	dfs = func(r, c int) {
		if r < 0 || r >= rows || c < 0 || c >= cols || grid[r][c] == '0' {
			return
		}

		grid[r][c] = '0'

		dfs(r+1, c)
		dfs(r-1, c)
		dfs(r, c+1)
		dfs(r, c-1)
	}

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if grid[r][c] == '1' {
				count++
				dfs(r, c)
			}
		}
	}
	return count
}

//Max Area of Island
/*
You are given an m x n binary matrix grid. An island is a group of 1's (representing land) connected
4-directionally (horizontal or vertical.) You may assume all four edges of the grid are surrounded by water.
The area of an island is the number of cells with a value 1 in the island.
Return the maximum area of an island in grid. If there is no island, return 0.

Example 1:
Input: grid = [[0,0,1,0,0,0,0,1,0,0,0,0,0],[0,0,0,0,0,0,0,1,1,1,0,0,0],[0,1,1,0,1,0,0,0,0,0,0,0,0],
[0,1,0,0,1,1,0,0,1,0,1,0,0],[0,1,0,0,1,1,0,0,1,1,1,0,0],[0,0,0,0,0,0,0,0,0,0,1,0,0],
[0,0,0,0,0,0,0,1,1,1,0,0,0],[0,0,0,0,0,0,0,1,1,0,0,0,0]]
Output: 6
Explanation: The answer is not 11, because the island must be connected 4-directionally.

Example 2:
Input: grid = [[0,0,0,0,0,0,0,0]]
Output: 0
*/
func maxAreaOfIsland(grid [][]int) int {
	if len(grid) == 0 {
		return 0
	}

	rows := len(grid)
	cols := len(grid[0])
	maxArea := 0

	var dfs func(r, c int) int
	dfs = func(r, c int) int {
		if r < 0 || r >= rows || c < 0 || c >= cols || grid[r][c] == 0 {
			return 0
		}
		grid[r][c] = 0
		return 1 + dfs(r+1, c) + dfs(r-1, c) + dfs(r, c+1) + dfs(r, c-1)
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if grid[r][c] == 1 {
				area := dfs(r, c)
				if area > maxArea {
					maxArea = area
				}
			}
		}
	}
	return maxArea
}

//Rotting Oranges
/*
You are given an m x n grid where each cell can have one of three values:
0 representing an empty cell,
1 representing a fresh orange, or
2 representing a rotten orange.
Every minute, any fresh orange that is 4-directionally adjacent to a rotten orange becomes rotten.
Return the minimum number of minutes that must elapse until no cell has a fresh orange. If this is impossible, return -1.

Example 1:
Input: grid = [[2,1,1],[1,1,0],[0,1,1]]
Output: 4

Example 2:
Input: grid = [[2,1,1],[0,1,1],[1,0,1]]
Output: -1
Explanation: The orange in the bottom left corner (row 2, column 0) is never rotten, because rotting only happens 4-directionally.

Example 3:
Input: grid = [[0,2]]
Output: 0
Explanation: Since there are already no fresh oranges at minute 0, the answer is just 0.
*/
func orangesRotting(grid [][]int) int {
	if len(grid) == 0 {
		return 0
	}

	rows := len(grid)
	cols := len(grid[0])

	queue := [][2]int{}
	freshCount := 0
	min := 0

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if grid[r][c] == 2 {
				queue = append(queue, [2]int{r, c})
			} else if grid[r][c] == 1 {
				freshCount++
			}
		}
	}
	dirs := [][]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for len(queue) > 0 && freshCount > 0 {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			curr := queue[0]
			queue = queue[1:]
			r, c := curr[0], curr[1]

			for _, d := range dirs {
				nr, nc := r+d[0], c+d[1]

				if nr >= 0 && nr < rows && nc >= 0 && nc < cols && grid[nr][nc] == 1 {
					grid[nr][nc] = 2
					freshCount--
					queue = append(queue, [2]int{nr, nc})
				}
			}
		}
		min++
	}

	if freshCount > 0 {
		return -1
	}
	return min
}

//Clone Graph
/*
Given a reference of a node in a connected undirected graph.
Return a deep copy (clone) of the graph.
Each node in the graph contains a value () and a list () of its neighbors.intList[Node]

class Node {
    public int val;
    public List<Node> neighbors;
}

Test case format:
For simplicity, each node's value is the same as the node's index (1-indexed). For example, the first node with,
the second node with , and so on. The graph is represented in the test case using an adjacency list.val == 1val == 2
An adjacency list is a collection of unordered lists used to represent a finite graph. Each list describes the set of neighbors of a node in the graph.
The given node will always be the first node with . You must return the copy of the given node as a reference to the cloned graph.val = 1

Example 1:
Input: adjList = [[2,4],[1,3],[2,4],[1,3]]
Output: [[2,4],[1,3],[2,4],[1,3]]
Explanation: There are 4 nodes in the graph.
1st node (val = 1)'s neighbors are 2nd node (val = 2) and 4th node (val = 4).
2nd node (val = 2)'s neighbors are 1st node (val = 1) and 3rd node (val = 3).
3rd node (val = 3)'s neighbors are 2nd node (val = 2) and 4th node (val = 4).
4th node (val = 4)'s neighbors are 1st node (val = 1) and 3rd node (val = 3).

Example 2:
Input: adjList = [[]]
Output: [[]]
Explanation: Note that the input contains one empty list. The graph consists of only one node with val = 1 and it does not have any neighbors.

Example 3:
Input: adjList = []
Output: []
Explanation: This an empty graph, it does not have any nodes.
*/
func cloneGraph(node *Node) *Node {
	visited := make(map[*Node]*Node)

	var dfs func(n *Node) *Node
	dfs = func(n *Node) *Node {
		if n == nil {
			return nil
		}

		if clone, ok := visited[n]; ok {
			return clone
		}

		clone := &Node{Val: n.Val}
		visited[n] = clone

		for _, neighbor := range n.Neighbors {
			clone.Neighbors = append(clone.Neighbors, dfs(neighbor))
		}
		return clone
	}
	return dfs(node)
}

//Course Schedule
/*
There are a total of courses you have to take, labeled from to . You are given an array where
indicates that you must take course first if you want to take course .numCourses0numCourses - 1prerequisitesprerequisites[i] = [ai, bi]biai
For example, the pair , indicates that to take course you have to first take course .[0, 1]01
Return if you can finish all courses. Otherwise, return .truefalse

Example 1:
Input: numCourses = 2, prerequisites = [[1,0]]
Output: true
Explanation: There are a total of 2 courses to take.
To take course 1 you should have finished course 0. So it is possible.

Example 2:
Input: numCourses = 2, prerequisites = [[1,0],[0,1]]
Output: false
Explanation: There are a total of 2 courses to take.
To take course 1 you should have finished course 0, and to take course 0 you should also have finished course 1. So it is impossible.
*/
func canFinish(numCourses int, prerequisites [][]int) bool {
	graph := make([][]int, numCourses)
	inDegree := make([]int, numCourses)

	for _, pre := range prerequisites {
		course := pre[0]
		prereq := pre[1]

		graph[prereq] = append(graph[prereq], course)
		inDegree[course]++
	}

	queue := []int{}
	for i := 0; i < numCourses; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	completed := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		completed++

		for _, neighbor := range graph[curr] {
			inDegree[neighbor]--

			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}
	return completed == numCourses
}

//Number of Provinces
/*
There are n cities. Some of them are connected, while some are not. If city a is connected directly with city b,
and city b is connected directly with city c, then city a is connected indirectly with city c.
A province is a group of directly or indirectly connected cities and no other cities outside of the group.
You are given an n x n matrix isConnected where isConnected[i][j] = 1 if the ith city and the jth city are
directly connected, and isConnected[i][j] = 0 otherwise. Return the total number of provinces.

Example 1:
Input: isConnected = [[1,1,0],[1,1,0],[0,0,1]]
Output: 2

Example 2:
Input: isConnected = [[1,0,0],[0,1,0],[0,0,1]]
Output: 3
*/
func findCircleNum(isConnected [][]int) int {
	n := len(isConnected)
	parent := make([]int, n)

	for i := 0; i < n; i++ {
		parent[i] = i
	}

	count := n

	var find func(i int) int
	find = func(i int) int {
		if parent[i] == i {
			return i
		}
		parent[i] = find(parent[i])
		return parent[i]
	}

	union := func(i, j int) {
		rootI := find(i)
		rootJ := find(j)

		if rootI != rootJ {
			parent[rootI] = rootJ
			count--
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if isConnected[i][j] == 1 {
				union(i, j)
			}
		}
	}
	return count
}

//Count Connected Components in LCM Graph
/*
You are given an array of integers nums of size n and a positive integer threshold.
There is a graph consisting of n nodes with the ith node having a value of nums[i]. Two nodes i and j in the
graph are connected via an undirected edge if lcm(nums[i], nums[j]) <= threshold.
Return the number of connected components in this graph.
A connected component is a subgraph of a graph in which there exists a path between any two vertices,
and no vertex of the subgraph shares an edge with a vertex outside of the subgraph.
The term lcm(a, b) denotes the least common multiple of a and b.

Example 1:
Input: nums = [2,4,8,3,9], threshold = 5
Output: 4
The four connected components are (2, 4), (3), (8), (9).

Example 2:
Input: nums = [2,4,8,3,9,12], threshold = 10
Output: 2
The two connected components are (2, 3, 4, 8, 9), and (12).
*/
type DSU struct {
	parent []int
	rank   []int
}

func NewDSU(n int) *DSU {
	parent := make([]int, n+1)
	rank := make([]int, n+1)
	for i := 0; i <= n; i++ {
		parent[i] = i
		rank[i] = 0
	}
	return &DSU{parent: parent, rank: rank}
}

func (d *DSU) Find(x int) int {
	if d.parent[x] != x {
		d.parent[x] = d.Find(d.parent[x])
	}
	return d.parent[x]
}

func (d *DSU) Union(x, y int) bool {
	rootX := d.Find(x)
	rootY := d.Find(y)

	if rootX != rootY {
		if d.rank[rootX] < d.rank[rootY] {
			d.parent[rootX] = rootY
		} else if d.rank[rootX] > d.rank[rootY] {
			d.parent[rootY] = rootX
		} else {
			d.parent[rootY] = rootX
			d.rank[rootX]++
		}
		return true
	}
	return false
}

func countComponents(nums []int, threshold int) int {
	dsu := NewDSU(threshold + 1)

	inNums := make([]bool, threshold+1)
	for _, num := range nums {
		if num <= threshold {
			inNums[num] = true
		}
	}

	for g := 1; g <= threshold; g++ {
		var minX int = -1
		for x := g; x <= threshold; x += g {
			if inNums[x] {
				minX = x
				break
			}
		}

		if minX == -1 {
			continue
		}

		for y := minX + g; y <= threshold && y*minX <= g*threshold; y += g {
			if inNums[y] {
				dsu.Union(minX, y)
			}
		}
	}
	uniqueRoots := make(map[int]bool)
	for _, num := range nums {
		if num > threshold {
			uniqueRoots[num] = true
		} else {
			root := dsu.Find(num)
			uniqueRoots[root] = true
		}
	}
	return len(uniqueRoots)
}

//Is Graph Bipartite?
/*
There is an undirected graph with n nodes, where each node is numbered between 0 and n - 1. You are given a 2D array graph,
where graph[u] is an array of nodes that node u is adjacent to. More formally, for each v in graph[u], there is an
undirected edge between node u and node v. The graph has the following properties:
There are no self-edges (graph[u] does not contain u).
There are no parallel edges (graph[u] does not contain duplicate values).
If v is in graph[u], then u is in graph[v] (the graph is undirected).
The graph may not be connected, meaning there may be two nodes u and v such that there is no path between them.
A graph is bipartite if the nodes can be partitioned into two independent sets A and B such that every
edge in the graph connects a node in set A and a node in set B. Return true if and only if it is bipartite.

Example 1:
Input: graph = [[1,2,3],[0,2],[0,1,3],[0,2]]
Output: false
Explanation: There is no way to partition the nodes into two independent sets
such that every edge connects a node in one and a node in the other.

Example 2:
Input: graph = [[1,3],[0,2],[1,3],[0,2]]
Output: true
Explanation: We can partition the nodes into two sets: {0, 2} and {1, 3}.
*/
func isBipartite(graph [][]int) bool {
	n := len(graph)

	colors := make([]int, n)

	var dfs func(node, color int) bool
	dfs = func(node, color int) bool {
		colors[node] = color

		for _, neig := range graph[node] {
			if colors[neig] == 0 {
				if !dfs(neig, -color) {
					return false
				}
			} else if colors[neig] == color {
				return false
			}
		}
		return true
	}

	for i := 0; i < n; i++ {
		if colors[i] == 0 {
			if !dfs(i, 1) {
				return false
			}
		}
	}
	return true
}

//Shortest Path in Binary Matrix
/*
Given an n x n binary matrix grid, return the length of the shortest clear path in the matrix. If there is no clear path, return -1.
A clear path in a binary matrix is a path from the top-left cell (i.e., (0, 0)) to the bottom-right cell (i.e., (n - 1, n - 1)) such that:
All the visited cells of the path are 0.
All the adjacent cells of the path are 8-directionally connected (i.e., they are different and they share an edge or a corner).
The length of a clear path is the number of visited cells of this path.

Example 1:
Input: grid = [[0,1],[1,0]]
Output: 2

Example 2:
Input: grid = [[0,0,0],[1,1,0],[1,1,0]]
Output: 4

Example 3:
Input: grid = [[1,0,0],[1,1,0],[1,1,0]]
Output: -1
*/
func shortestPathBinaryMatrix(grid [][]int) int {
	n := len(grid)

	if grid[0][0] == 1 || grid[n-1][n-1] == 1 {
		return -1
	}

	queue := [][2]int{{0, 0}}
	grid[0][0] = 1

	dirs := [][]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}

	pathLen := 1

	for len(queue) > 0 {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			curr := queue[0]
			queue = queue[1:]
			r, c := curr[0], curr[1]

			if r == n-1 && c == n-1 {
				return pathLen
			}

			for _, d := range dirs {
				nr, nc := r+d[0], c+d[1]

				if nr >= 0 && nr < n && nc >= 0 && nc < n && grid[nr][nc] == 0 {
					grid[nr][nc] = 1
					queue = append(queue, [2]int{nr, nc})
				}
			}
		}
		pathLen++
	}
	return -1
}
