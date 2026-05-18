package main

import (
	"fmt"
	"math/rand"
)

func main() {
	fmt.Println(sum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursinSum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursinSumInd([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursionMaxNum([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recursionMax([]int{2, 6, 5, 2, 1, 9}))
	fmt.Println(recBinSearch([]int{2, 6, 5, 2, 1, 9}, 5, 0, 5))
	fmt.Println(quickSort([]int{2, 5, 1, 8, 10, 51, 22, 14, 50}))
	arr := []int{2, 3, 7, 8, 10}
	table := multiTable(arr)
	printTable(arr, table)
}

func sum(sl []int) int {
	var result int
	for _, val := range sl {
		result += val
	}
	return result
}

func recursinSum(sl []int) int {
	if len(sl) == 0 {
		return 0
	}

	return sl[0] + recursinSum(sl[1:])
}

func recursinSumInd(sl []int) int {
	if len(sl) == 0 {
		return 0
	}

	return 1 + recursinSumInd(sl[1:])
}

func recursionMaxNum(sl []int) int {
	if len(sl) == 1 {
		return sl[0]
	}
	if sl[0] > sl[1] {
		sl[1] = sl[0]
		return recursionMaxNum(sl[1:])
	}
	return recursionMaxNum(sl[1:])
}

func recursionMax(sl []int) int {
	if len(sl) == 1 {
		return sl[0]
	}

	subMax := recursionMax(sl[1:])

	if sl[0] > subMax {
		return sl[0]
	}
	return subMax
}

func recBinSearch(sl []int, val int, low int, high int) int {
	if low > high {
		return -1
	}

	mid := (low + high) / 2
	if sl[mid] == val {
		return mid
	}
	if sl[mid] > val {
		return recBinSearch(sl, val, low, mid-1)
	} else {
		return recBinSearch(sl, val, mid+1, high)
	}
}

func quickSort(arr []int) []int {
	if len(arr) < 2 {
		return arr
	}
	var (
		less    []int
		greater []int
	)

	pivotInd := rand.Intn(len(arr))
	pivot := arr[pivotInd]

	for i, val := range arr {
		if i == pivotInd {
			continue
		}
		if val < pivot {
			less = append(less, val)
		} else {
			greater = append(greater, val)
		}
	}

	result := append(quickSort(less), pivot)
	result = append(result, quickSort(greater)...)
	return result
}

func multiTable(arr []int) [][]int {
	// Создаем пустую матрицу (слайс слайсов)
	var table [][]int

	// Внешний цикл: бежит по строкам (множитель 1)
	for _, num1 := range arr {
		var row []int // Для каждой новой строки создаем свой пустой слайс

		// Внутренний цикл: бежит по столбцам (множитель 2)
		for _, num2 := range arr {
			result := num1 * num2
			row = append(row, result) // Складываем результаты в текущую строку
		}

		// Когда строка готова, добавляем её в нашу большую таблицу
		table = append(table, row)
	}

	return table
}

func printTable(arr []int, table [][]int) {
	// 1. Печатаем верхнюю «шапку» (пропускаем первое место для угла таблицы)
	fmt.Printf("     ") // 5 пробелов, чтобы выровнять с боковой панелью
	for _, num := range arr {
		fmt.Printf("%4d ", num)
	}
	fmt.Println()

	// Рисуем разделительную линию для красоты
	fmt.Println("     -------------------------")

	// 2. Печатаем строки таблицы
	for i, row := range table {
		// Печатаем боковое число из исходного массива
		fmt.Printf("%4d |", arr[i])

		// Печатаем сами результаты умножения
		for _, val := range row {
			fmt.Printf("%4d ", val)
		}
		fmt.Println()
	}
}
