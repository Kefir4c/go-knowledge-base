package __bst

/*
  АЛГОРИТМЫ 9: BINARY SEARCH TREE (BST)
  Binary Search Tree (BST) — это бинарное дерево с жёстким правилом: для любого узла все значения в левом поддереве меньше него, а все значения в правом поддереве — больше.
  Это правило превращает обычное дерево в мощный инструмент поиска, вставки и удаления за O(log n) в сбалансированном случае.
  На собесе BST — это проверка на понимание рекурсии, границ (bounds) и свойства in-order обхода (который выдаёт отсортированный массив).
  Если обычные деревья — это про обходы, то BST — это про использование свойств для отсечения половины дерева.

  СОДЕРЖАНИЕ:
    1.  Зачем нужен BST
    2.  Свойства BST (и как их проверять)
    3.  Поиск в BST (Search)
    4.  Вставка в BST (Insert)
    5.  Удаление из BST (Delete) — самое сложное
    5.1 LCA в BST (использование свойства)
    6.  Магия In-order обхода в BST
    6.1 Kth Smallest Element + BST Iterator
    6.2 Floor / Ceil / Successor / Predecessor
    7.  Пример 1: Delete Node in a BST
    8.  Пример 2: Trim a Binary Search Tree
    9.  Пример 3: Convert Sorted Array to BST
    10. Пример 4: Minimum Absolute Difference in BST
    10.1 Range Sum of BST (прунинг)
    10.2 Balanced BST Check
    10.3 BST vs Array vs Hash Map
    11. Общие правила и ловушки
    12. Сложность
    13. Связь с Go
    14. Частые ловушки
    15. Антипаттерны
    16. Финальные выводы

  1. ЗАЧЕМ НУЖЕН BST
  ПРОБЛЕМА: в обычном бинарном дереве, чтобы найти элемент, нужно обойти все узлы (O(n)). Массив отсортирован, но вставка в середину — O(n).
  РЕШЕНИЕ: BST. Мы упорядочиваем узлы по правилу: «левое меньше, правое больше». Теперь на каждом шаге мы отсекаем половину дерева, как в бинарном поиске.

  ЧТО ЭТО ДАЁТ:
    • Поиск, вставка, удаление за O(log n) в среднем.
    • In-order обход выдаёт элементы в отсортированном порядке.
    • Легко находить минимум (самый левый) и максимум (самый правый).
    • Основа для TreeMap, B-Tree (в базах данных), красно-чёрных деревьев.

  КОГДА ИСПОЛЬЗОВАТЬ:
    • Задача явно просит «Binary Search Tree».
    • Нужно найти K-й элемент, ближайший элемент, диапазон.
    • Входной массив уже отсортирован, нужно построить дерево.

  ПРАВИЛО: если задача даёт BST, ты почти всегда используешь свойство «левое < корень < правое» для отсечения веток.

  2. СВОЙСТВА BST (И КАК ИХ ПРОВЕРЯТЬ)

  ГЛАВНОЕ ПРАВИЛО:
    Для любого узла X:
    • Все значения в левом поддереве X строго меньше X.
    • Все значения в правом поддереве X строго больше X.
    • Это правило применяется рекурсивно к каждому узлу.

  ЛОВУШКА ПРИ ПРОВЕРКЕ (Валидация BST):
    Многие думают, что достаточно проверить: `left.Val < root.Val < right.Val`. Но это неверно!
    Узел в правом поддереве может быть больше своего родителя, но меньше «дедушки».

    Пример невалидного BST:
          5
         / \
        1   6
           / \
          3   7
    Здесь `6` больше `5` (ок), `3` меньше `6` (ок). Но `3` в правом поддереве `5`, значит, `3` должно быть больше `5`. Это не так.

  КАК ПРАВИЛЬНО ПРОВЕРЯТЬ:
    Передаём диапазон допустимых значений (bounds) вниз по рекурсии.
    • Для корня: `(-inf, +inf)`.
    • Для левого ребёнка: `(low, root.Val)`.
    • Для правого ребёнка: `(root.Val, high)`.

    ШАБЛОН:
      func isValidBST(root *TreeNode) bool {
          var validate func(node *TreeNode, low, high *int) bool
          validate = func(node *TreeNode, low, high *int) bool {
              if node == nil { return true }
              if (low != nil && node.Val <= *low) || (high != nil && node.Val >= *high) { return false }
              return validate(node.Left, low, &node.Val) && validate(node.Right, &node.Val, high)
          }
          return validate(root, nil, nil)
      }

  ПРАВИЛО: BST — это про границы (bounds). Всегда думай, какие ограничения накладывают на тебя предки.

  3. ПОИСК В BST (SEARCH)

  ИДЕЯ:
    • Сравниваем target с текущим узлом.
    • Если равно — нашли.
    • Если меньше — идём влево (там значения меньше).
    • Если больше — идём вправо (там значения больше).

  РЕКУРСИВНО:
    func searchBST(root *TreeNode, val int) *TreeNode {
        if root == nil || root.Val == val { return root }
        if val < root.Val { return searchBST(root.Left, val) }
        return searchBST(root.Right, val)
    }

  ИТЕРАТИВНО (в Go часто предпочтительнее, нет оверхеда):
    func searchBST(root *TreeNode, val int) *TreeNode {
        curr := root
        for curr != nil {
            if val == curr.Val { return curr }
            if val < curr.Val { curr = curr.Left } else { curr = curr.Right }
        }
        return nil
    }

  ПРАВИЛО: поиск в BST — O(log n) в сбалансированном дереве и O(n) в вырожденном.

  4. ВСТАВКА В BST (INSERT)

  ИДЕЯ:
    • Идём как при поиске.
    • Когда натыкаемся на nil, вставляем новый узел.
    • Новый узел всегда становится листом.

  КОД:
    func insertIntoBST(root *TreeNode, val int) *TreeNode {
        if root == nil { return &TreeNode{Val: val} }
        if val < root.Val {
            root.Left = insertIntoBST(root.Left, val)
        } else {
            root.Right = insertIntoBST(root.Right, val)
        }
        return root
    }

  ВАЖНО:
    • В BST обычно нет дубликатов. Если разрешены — уточни, в какую сторону их класть.
    • Вставка не меняет структуру выше (кроме добавления листа).

  ПРАВИЛО: вставка — O(h), где h — высота дерева.

  5. УДАЛЕНИЕ ИЗ BST (DELETE) — САМОЕ СЛОЖНОЕ
  Удаление — единственная операция, которая может изменить структуру. Три случая:

  СЛУЧАЙ 1: Узел — лист (нет детей).
    Просто удаляем (возвращаем nil наверх).

  СЛУЧАЙ 2: Узел имеет одного ребёнка.
    Возвращаем наверх этого ребёнка (он занимает место удалённого).

  СЛУЧАЙ 3: Узел имеет двух детей.
    Нельзя просто удалить — нужно сохранить свойства BST.
    Находим преемника (наименьший узел в правом поддереве) или предшественника (наибольший в левом).
    Копируем значение преемника в удаляемый узел, а затем удаляем преемника (он будет иметь максимум одного ребёнка).

  КОД (поиск преемника):
    func findMin(node *TreeNode) *TreeNode {
        for node.Left != nil { node = node.Left }
        return node
    }

  ПРАВИЛО: при удалении узла с двумя детьми заменяем его значение минимальным из правого поддерева, а затем рекурсивно удаляем преемника.

  5.1 LCA В BST (ИСПОЛЬЗОВАНИЕ СВОЙСТВА)

  ЗАДАЧА: найти Lowest Common Ancestor (LCA) двух узлов в BST. Это классика, и в BST она решается принципиально проще, чем в обычном дереве.

  ИДЕЯ:
    • Мы используем свойство порядка, чтобы не обходить всё дерево.
    • Если оба узла меньше root — LCA точно в левом поддереве.
    • Если оба узла больше root — LCA точно в правом поддереве.
    • Иначе (один слева, другой справа, или root равен одному из них) — root и есть LCA.

  КОД:
    func lowestCommonAncestor(root, p, q *TreeNode) *TreeNode {
        curr := root
        for curr != nil {
            if p.Val < curr.Val && q.Val < curr.Val {
                curr = curr.Left
            } else if p.Val > curr.Val && q.Val > curr.Val {
                curr = curr.Right
            } else {
                return curr
            }
        }
        return nil
    }

  ПОЧЕМУ РАБОТАЕТ:
    • В обычном дереве мы делаем Postorder O(n), потому что не знаем, где узлы.
    • В BST мы знаем: если оба меньше — они точно слева. Значит, отсекаем половину.
    • Сложность O(h) — в сбалансированном это O(log n).

  СЛОЖНОСТЬ: O(h) time, O(1) space (итеративно).

  ПРАВИЛО: если задача про LCA в BST — не решай рекурсией, как в обычном дереве. Используй свойство порядка и итерацию.

  6. МАГИЯ IN-ORDER ОБХОДА В BST
  Если ты делаешь In-order обход (лево, узел, право) в BST, ты получаешь отсортированный по возрастанию массив.
  Это свойство используется в 50% задач на BST:
    • Kth Smallest Element (идём in-order и считаем до K).
    • Validate BST (сравниваем текущий с предыдущим).
    • Minimum Absolute Difference (сравниваем соседей).
    • Преобразование BST в отсортированный двусвязный список.

  ШАБЛОН (с сохранением prev):
    var prev *TreeNode
    var inorder func(node *TreeNode)
    inorder = func(node *TreeNode) {
        if node == nil { return }
        inorder(node.Left)
        if prev != nil {
            // Логика с prev и node
        }
        prev = node
        inorder(node.Right)
    }

  ПРАВИЛО: если задача про «K-й элемент», «отсортированный порядок» или «разницу» — это In-order.

  6.1 KTH SMALLEST ELEMENT + BST ITERATOR
  ЗАДАЧА (Kth Smallest): найти K-й наименьший элемент в BST.

  ИДЕЯ: In-order обход — но остановиться, когда счётчик дошёл до K.
  Сложность: O(h + k). Мы не обходим всё дерево, только K левых шагов.

  КОД:
    func kthSmallest(root *TreeNode, k int) int {
        stack := []*TreeNode{}
        curr := root
        for curr != nil || len(stack) > 0 {
            for curr != nil {
                stack = append(stack, curr)
                curr = curr.Left
            }
            curr = stack[len(stack)-1]
            stack = stack[:len(stack)-1]
            k--
            if k == 0 { return curr.Val }
            curr = curr.Right
        }
        return -1
    }

  ЗАДАЧА (BST Iterator): реализовать класс, который возвращает следующий элемент BST по запросу.
  Это самый сложный вариант — здесь нужен контролируемый In-order через стек, но без рекурсии.

  КОД (структура):
    type BSTIterator struct {
        stack []*TreeNode
    }

    func Constructor(root *TreeNode) BSTIterator {
        iter := BSTIterator{}
        iter.pushLeft(root)
        return iter
    }

    func (it *BSTIterator) pushLeft(node *TreeNode) {
        for node != nil {
            it.stack = append(it.stack, node)
            node = node.Left
        }
    }

    func (it *BSTIterator) Next() int {
        node := it.stack[len(it.stack)-1]
        it.stack = it.stack[:len(it.stack)-1]
        it.pushLeft(node.Right)
        return node.Val
    }

    func (it *BSTIterator) HasNext() bool {
        return len(it.stack) > 0
    }

  ПОЧЕМУ РАБОТАЕТ:
    • Стек имитирует рекурсию, но у нас есть контроль: Next() возвращает ровно один элемент.
    • В стеке всегда лежит путь «слева направо» — вершина стека это следующий по порядку элемент.
    • amortized O(1) на Next(), O(h) памяти.

  ПРАВИЛО: если задача требует «следующий по порядку элемент» — это BST Iterator. Стек, не рекурсия.

  6.2 FLOOR / CEIL / SUCCESSOR / PREDECESSOR
  Эти четыре операции — база для задач «найди ближайший», «найди следующий», «найди предыдущий». Все они используют свойство BST для движения вниз.
  FLOOR — наибольшее значение ≤ target.
  CEIL — наименьшее значение ≥ target.
  SUCCESSOR — первый узел со значением > target (следующий по порядку).
  PREDECESSOR — первый узел со значением < target (предыдущий по порядку).

  ШАБЛОН (FLOOR):
    func floor(root *TreeNode, target int) int {
        res := -1
        curr := root
        for curr != nil {
            if curr.Val == target {
                return curr.Val
            } else if curr.Val < target {
                res = curr.Val    // потенциальный кандидат
                curr = curr.Right // попробуем найти больше (но всё ещё ≤ target)
            } else {
                curr = curr.Left  // слишком большое, идём влево
            }
        }
        return res
    }

  ЧТО МЕНЯЕТСЯ ОТ ОПЕРАЦИИ К ОПЕРАЦИИ:
    • Floor: обновляем res, когда `curr.Val < target`, идём вправо.
    • Ceil: обновляем res, когда `curr.Val > target`, идём влево.
    • Successor: обновляем res, когда `curr.Val > target`, идём влево (как Ceil, но строго >).
    • Predecessor: обновляем res, когда `curr.Val < target`, идём вправо (как Floor, но строго <).

  СЛОЖНОСТЬ: O(h) time, O(1) space.

  ПРАВИЛО: Floor/Ceil/Successor/Predecessor — это один и тот же цикл с разными условиями обновления `res`. Запомни шаблон — 4 задачи одним кодом.

  7. ПРИМЕР 1: DELETE NODE IN A BST
  ЗАДАЧА: удалить узел с заданным ключом в BST.

  ИДЕЯ:
    1. Найти узел.
    2. Если нет детей — вернуть nil.
    3. Если один ребёнок — вернуть его.
    4. Если два ребёнка — найти минимум в правом поддереве, заменить значение, удалить минимум.

  КОД:
    func deleteNode(root *TreeNode, key int) *TreeNode {
        if root == nil { return nil }
        if key < root.Val {
            root.Left = deleteNode(root.Left, key)
        } else if key > root.Val {
            root.Right = deleteNode(root.Right, key)
        } else {
            if root.Left == nil { return root.Right }
            if root.Right == nil { return root.Left }
            minNode := root.Right
            for minNode.Left != nil { minNode = minNode.Left }
            root.Val = minNode.Val
            root.Right = deleteNode(root.Right, minNode.Val)
        }
        return root
    }

  СЛОЖНОСТЬ: O(h) time, O(h) space (стек рекурсии).

  ПРАВИЛО: удаление — единственный случай, где рекурсия возвращает модифицированное поддерево и присваивается обратно.

  8. ПРИМЕР 2: TRIM A BINARY SEARCH TREE
  ЗАДАЧА: обрезать дерево так, чтобы все значения были в диапазоне [low, high]. Дерево должно остаться BST.

  ИДЕЯ:
    • Если `node.Val < low` — узел и всё его левое поддерево меньше low. Возвращаем `trim(node.Right)`.
    • Если `node.Val > high` — узел и всё правое поддерево больше high. Возвращаем `trim(node.Left)`.
    • Иначе — узел в диапазоне. Рекурсивно обрезаем детей.

  КОД:
    func trimBST(root *TreeNode, low int, high int) *TreeNode {
        if root == nil { return nil }
        if root.Val < low { return trimBST(root.Right, low, high) }
        if root.Val > high { return trimBST(root.Left, low, high) }
        root.Left = trimBST(root.Left, low, high)
        root.Right = trimBST(root.Right, low, high)
        return root
    }

  СЛОЖНОСТЬ: O(n) time (в худшем), O(h) space.

  ПРАВИЛО: если узел вне диапазона — отсекаем целое поддерево с одной стороны. Это O(1) на узел.

  9. ПРИМЕР 3: CONVERT SORTED ARRAY TO BST
  ЗАДАЧА: дан отсортированный массив. Построить сбалансированное BST.

  ИДЕЯ:
    • Корень — средний элемент массива. Это гарантирует баланс.
    • Левая половина — левое поддерево. Правая — правое.

  КОД:
    func sortedArrayToBST(nums []int) *TreeNode {
        var build func(left, right int) *TreeNode
        build = func(left, right int) *TreeNode {
            if left > right { return nil }
            mid := left + (right-left)/2
            root := &TreeNode{Val: nums[mid]}
            root.Left = build(left, mid-1)
            root.Right = build(mid+1, right)
            return root
        }
        return build(0, len(nums)-1)
    }

  СЛОЖНОСТЬ: O(n) time, O(log n) space.

  ПРАВИЛО: для сбалансированного BST из отсортированного массива всегда бери средний элемент как корень.

  10. ПРИМЕР 4: MINIMUM ABSOLUTE DIFFERENCE IN BST
  ЗАДАЧА: найти минимальную абсолютную разницу между значениями любых двух узлов.

  ИДЕЯ:
    • In-order обход даёт отсортированные значения.
    • Минимальная разница — всегда между соседями.
    • Ведём `prev` и считаем разницу с текущим.

  КОД:
    func getMinimumDifference(root *TreeNode) int {
        minDiff := math.MaxInt32
        var prev *TreeNode
        var inorder func(node *TreeNode)
        inorder = func(node *TreeNode) {
            if node == nil { return }
            inorder(node.Left)
            if prev != nil {
                diff := node.Val - prev.Val
                if diff < minDiff { minDiff = diff }
            }
            prev = node
            inorder(node.Right)
        }
        inorder(root)
        return minDiff
    }

  СЛОЖНОСТЬ: O(n) time, O(h) space.

  ПРАВИЛО: если задача про «разницу» или «ближайший» — In-order + отслеживание `prev`.

  10.1 RANGE SUM OF BST (ПРУНИНГ)
  ЗАДАЧА: найти сумму всех значений в BST, которые попадают в диапазон [low, high].

  ИДЕЯ:
    • Не надо обходить всё дерево. Используем свойство BST для отсечения.
    • Если `node.Val < low` — левое поддерево тоже меньше low. Идём только вправо.
    • Если `node.Val > high` — правое поддерево тоже больше high. Идём только влево.
    • Иначе — узел в диапазоне. Суммируем и идём в обе стороны.

  КОД:
    func rangeSumBST(root *TreeNode, low int, high int) int {
        if root == nil { return 0 }
        if root.Val < low { return rangeSumBST(root.Right, low, high) }
        if root.Val > high { return rangeSumBST(root.Left, low, high) }
        return root.Val + rangeSumBST(root.Left, low, high) + rangeSumBST(root.Right, low, high)
    }

  СЛОЖНОСТЬ: O(n) worst case, но в среднем лучше — отсекаем поддеревья.

  ПРАВИЛО: если узел вне диапазона, всё поддерево с одной стороны тоже вне диапазона. Отсекай.

  10.2 BALANCED BST CHECK
  ЗАДАЧА: проверить, является ли дерево сбалансированным по высоте (разница высот левого и правого поддерева для каждого узла ≤ 1).

  ИДЕЯ:
    • Postorder: дети возвращают свою высоту или -1 (если уже не сбалансировано).
    • Если левый или правый вернул -1 — пробрасываем -1 наверх.
    • Если разница высот больше 1 — возвращаем -1.
    • Иначе возвращаем max(left, right) + 1.

  КОД:
    func isBalanced(root *TreeNode) bool {
        var check func(node *TreeNode) int
        check = func(node *TreeNode) int {
            if node == nil { return 0 }
            left := check(node.Left)
            right := check(node.Right)
            if left == -1 || right == -1 { return -1 }
            if left-right > 1 || right-left > 1 { return -1 }
            if left > right { return left + 1 }
            return right + 1
        }
        return check(root) != -1
    }

  СЛОЖНОСТЬ: O(n) time, O(h) space.

  ЗАЧЕМ ЭТО ЗНАТЬ:
    • Именно балансировка гарантирует O(log n) в BST.
    • В вырожденном дереве (бамбук) высота = n, и все операции становятся O(n).
    • AVL и Red-Black деревья — это самобалансирующиеся BST. Проверка сбалансированности — прямая связь с этими структурами.

  ПРАВИЛО: balanced check — классический Postorder с «отравленным» возвратом -1 как флагом.

  10.3 BST VS ARRAY VS HASH MAP
  На собесе часто спрашивают: «Почему бы не использовать массив/map?». Нужно понимать трейд-оффы.

  SORTED ARRAY:
    • Поиск: O(log n) — binary search.
    • Вставка: O(n) — сдвиг элементов.
    • Удаление: O(n).
    • Плюсы: компактно в памяти, отличный кэш.
    • Минусы: медленные вставки/удаления.

  HASH MAP:
    • Поиск: O(1) в среднем.
    • Вставка: O(1).
    • Удаление: O(1).
    • Плюсы: быстрые операции.
    • Минусы: нет порядка, нельзя найти «следующий/предыдущий», нельзя диапазонные запросы.

  BST (СБАЛАНСИРОВАННОЕ):
    • Поиск: O(log n).
    • Вставка: O(log n).
    • Удаление: O(log n).
    • Плюсы: отсортированный порядок, range queries, next/prev, min/max за O(log n).
    • Минусы: медленнее map в среднем, память на указатели.

  КОГДА ЧТО:
    • Только быстрый поиск по ключу — Hash Map.
    • Массив, который редко меняется — Sorted Array + binary search.
    • Нужен порядок + быстрые операции + range queries — BST (или skip list, B-tree).
    • Нужен «K-й элемент», «следующий», «в диапазоне» — BST незаменим.

  ПРАВИЛО: если на собесе спрашивают «зачем BST, если есть map» — ответ: «Порядок. Map не даёт next/prev и range queries. BST даёт всё это за O(log n)».

  11. ОБЩИЕ ПРАВИЛА И ЛОВУШКИ

  КОГДА ИСПОЛЬЗОВАТЬ СВОЙСТВО BST:
    • Поиск, вставка, удаление.
    • Обрезка дерева (Trim, Range Sum).
    • Валидация.
    • LCA (можно без рекурсии, просто по значениям).
    • Floor/Ceil/Successor/Predecessor.

  КОГДА ИСПОЛЬЗОВАТЬ IN-ORDER:
    • K-й элемент.
    • Отсортированный вывод.
    • Минимальная разница.
    • BST Iterator.

  ЛОВУШКИ:
    • Проверка только `left.Val < root.Val < right.Val` (нужны границы).
    • Забыть, что при удалении узла с двумя детьми нужно найти преемника.
    • В Trim забыть, что если `root.Val < low`, всё левое поддерево тоже меньше low.
    • Использовать `<=` вместо `<` для BST с уникальными значениями (и наоборот).
    • Забыть про MathMin/MaxInt для границ в Go (или использовать `*int`).

  ПРАВИЛО: BST — это не просто дерево, это упорядоченная структура. Ищи монотонность и используй её.

  12. СЛОЖНОСТЬ

  СРЕДНЯЯ (СБАЛАНСИРОВАННОЕ ДЕРЕВО):
    • Поиск, вставка, удаление: O(log n).
    • In-order обход: O(n).
    • Floor/Ceil/Successor/Predecessor: O(log n).
    • LCA: O(log n).

  ХУДШАЯ (ВЫРОЖДЕННОЕ / БАМБУК):
    • Поиск, вставка, удаление: O(n).
    • In-order: O(n).
    • Все операции деградируют до O(n).

  ПАМЯТЬ:
    • Рекурсивные операции: O(h).
    • Итеративные операции: O(1).

  ПРАВИЛО: в реальных собесах BST считают сбалансированным, если не сказано иное, но всегда упоминай худший случай.

  13. СВЯЗЬ С GO

  ОСОБЕННОСТИ:
    • Go не имеет встроенного BST. Используется TreeNode с Left и Right.
    • Указатели и nil — основа.
    • Для границ в валидации: `math.MinInt64` / `math.MaxInt64` (проще) или `*int` (гибче).
    • In-order через замыкание с `prev` — стандартный паттерн.
    • BST Iterator — это структура со стеком и методом Next(). Показывает понимание ООП в Go.

  ПРИМЕР С math.MinInt64:
    func isValidBST(root *TreeNode) bool {
        var validate func(node *TreeNode, min, max int64) bool
        validate = func(node *TreeNode, min, max int64) bool {
            if node == nil { return true }
            val := int64(node.Val)
            if val <= min || val >= max { return false }
            return validate(node.Left, min, val) && validate(node.Right, val, max)
        }
        return validate(root, math.MinInt64, math.MaxInt64)
    }

  ПРАВИЛО: в Go для границ используй int64 с максимумами. Для итератора — структуру со стеком.

  14. ЧАСТЫЕ ЛОВУШКИ
    • Валидация только по родителю.
    • Забыть обновить указатели при удалении.
    • Не учитывать вырожденные деревья в сложности.
    • Не использовать свойство BST, а просто проходить всё дерево.

  15. АНТИПАТТЕРНЫ
  15.1. СРАВНЕНИЕ ТОЛЬКО С РОДИТЕЛЕМ.
    Не работает. Всегда передавай границы.
  15.2. ЗАБЫТЬ ПРО ПРЕЕМНИКА ПРИ УДАЛЕНИИ.
    Нарушит свойства BST.
  15.3. ИСПОЛЬЗОВАТЬ IN-ORDER, КОГДА НУЖЕН ПОИСК.
    Если дерево BST, поиск за O(log n). In-order — O(n).
  15.4. ПЫТАТЬСЯ СДЕЛАТЬ УДАЛЕНИЕ ИТЕРАТИВНО.
    Рекурсия читается лучше и обрабатывает все случаи.
  15.5. НЕ ПРОВЕРЯТЬ НА NIL.
    Всегда `if root == nil { return nil }`.
  15.6. ЗАБЫТЬ, ЧТО В BST ДУБЛИКАТЫ ОБЫЧНО НЕ РАЗРЕШЕНЫ.
    Если разрешены — уточни, в какую сторону их класть.
  15.7. РЕШАТЬ LCA В BST КАК В ОБЫЧНОМ ДЕРЕВЕ.
    Это O(n) вместо O(log n). Используй свойство BST.
  15.8. НЕ УПОМИНАТЬ БАЛАНСИРОВКУ.
    Если не сказать, что O(log n) только для сбалансированного, интервьюер может докопаться.

  16. ФИНАЛЬНЫЕ ВЫВОДЫ
  1.  BST — бинарное дерево с правилом: левое < корень < правое.
  2.  Валидация требует передачи границ (bounds) вниз, а не просто сравнения с родителем.
  3.  Поиск, вставка, удаление — O(log n) в сбалансированном, O(n) в вырожденном.
  4.  Удаление узла с двумя детьми — найти преемника (min в правом), заменить значение, удалить преемника.
  5.  LCA в BST — итеративно по значениям, O(log n). Не рекурсией, как в обычном дереве.
  6.  In-order обход в BST выдаёт отсортированный массив. Ключ к 50% задач.
  7.  Kth Smallest — In-order с остановкой на K. BST Iterator — контролируемый In-order через стек.
  8.  Floor/Ceil/Successor/Predecessor — один и тот же цикл с разными условиями обновления.
  9.  Trim и Range Sum — отсекаем целые поддеревья по свойству BST.
  10. Balanced check — Postorder с возвратом -1 как флагом.
  11. BST vs map: BST даёт порядок, next/prev, range queries. Map — быстрее, но без порядка.
  12. Для валидации в Go — math.MinInt64 / math.MaxInt64 или *int.
  13. Антипаттерны: сравнение только с родителем, забыть преемника, LCA рекурсией, не упоминать баланс.
*/
