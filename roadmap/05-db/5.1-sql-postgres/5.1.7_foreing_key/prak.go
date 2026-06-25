package __1_7_foreing_key

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// МОДЕЛИ

type Teacher struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type Course struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	TeacherID int64  `db:"teacher_id"`
}

type Student struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type Grade struct {
	ID        int64 `db:"id"`
	StudentID int64 `db:"student_id"`
	CourseID  int64 `db:"course_id"`
	Grade     int   `db:"grade"`
}

// TeacherStats — результат для топ-преподавателей
type TeacherStats struct {
	TeacherID   int64   `db:"teacher_id"`
	TeacherName string  `db:"teacher_name"`
	AvgGrade    float64 `db:"avg_grade"`
	GradeCount  int     `db:"grade_count"`
	CourseCount int     `db:"course_count"`
}

// РЕПОЗИТОРИЙ

type TeacherRepo struct {
	pool *pgxpool.Pool
}

func NewTeacherRepo(pool *pgxpool.Pool) *TeacherRepo {
	return &TeacherRepo{pool: pool}
}

// CRUD ДЛЯ TEACHERS

func (r *TeacherRepo) CreateTeacher(ctx context.Context, name string) (*Teacher, error) {
	var t Teacher
	query := `INSERT INTO teachers (name) VALUES ($1) RETURNING id, name`
	err := r.pool.QueryRow(ctx, query, name).Scan(&t.ID, &t.Name)
	if err != nil {
		return nil, fmt.Errorf("create teacher: %w", err)
	}
	return &t, nil
}

func (r *TeacherRepo) GetTeacherByID(ctx context.Context, id int64) (*Teacher, error) {
	var t Teacher
	query := `SELECT id, name FROM teachers WHERE id = $1`
	err := r.pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.Name)
	if err != nil {
		return nil, fmt.Errorf("get teacher: %w", err)
	}
	return &t, nil
}

// CRUD ДЛЯ COURSES

func (r *TeacherRepo) CreateCourse(ctx context.Context, name string, teacherID int64) (*Course, error) {
	var c Course
	query := `INSERT INTO courses (name, teacher_id) VALUES ($1, $2) RETURNING id, name, teacher_id`
	err := r.pool.QueryRow(ctx, query, name, teacherID).Scan(&c.ID, &c.Name, &c.TeacherID)
	if err != nil {
		return nil, fmt.Errorf("create course: %w", err)
	}
	return &c, nil
}

// CRUD ДЛЯ STUDENTS

func (r *TeacherRepo) CreateStudent(ctx context.Context, name string) (*Student, error) {
	var s Student
	query := `INSERT INTO students (name) VALUES ($1) RETURNING id, name`
	err := r.pool.QueryRow(ctx, query, name).Scan(&s.ID, &s.Name)
	if err != nil {
		return nil, fmt.Errorf("create student: %w", err)
	}
	return &s, nil
}

// CRUD ДЛЯ GRADES

func (r *TeacherRepo) CreateGrade(ctx context.Context, studentID, courseID int64, grade int) (*Grade, error) {
	var g Grade
	query := `INSERT INTO grades (student_id, course_id, grade) VALUES ($1, $2, $3) RETURNING id, student_id, course_id, grade`
	err := r.pool.QueryRow(ctx, query, studentID, courseID, grade).Scan(&g.ID, &g.StudentID, &g.CourseID, &g.Grade)
	if err != nil {
		return nil, fmt.Errorf("create grade: %w", err)
	}
	return &g, nil
}

// ОСНОВНОЙ МЕТОД: ТОП-5 ПРЕПОДАВАТЕЛЕЙ ПО СРЕДНЕЙ ОЦЕНКЕ

// GetTopTeachersByAvgGrade возвращает топ-5 преподавателей по средней оценке,
// исключая преподавателей с менее чем minCourses курсами.
func (r *TeacherRepo) GetTopTeachersByAvgGrade(ctx context.Context, minCourses int, limit int) ([]TeacherStats, error) {
	if minCourses < 1 {
		minCourses = 2
	}
	if limit < 1 {
		limit = 5
	}

	query := `
		WITH teacher_stats AS (
			SELECT
				t.id AS teacher_id,
				t.name AS teacher_name,
				AVG(g.grade) AS avg_grade,
				COUNT(g.id) AS grade_count,
				COUNT(DISTINCT c.id) AS course_count
			FROM teachers t
			JOIN courses c ON t.id = c.teacher_id
			JOIN grades g ON c.id = g.course_id
			GROUP BY t.id, t.name
			HAVING COUNT(DISTINCT c.id) >= $1
		)
		SELECT
			teacher_id,
			teacher_name,
			ROUND(avg_grade, 2) AS avg_grade,
			grade_count,
			course_count
		FROM teacher_stats
		ORDER BY avg_grade DESC, grade_count DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, minCourses, limit)
	if err != nil {
		return nil, fmt.Errorf("query top teachers: %w", err)
	}
	defer rows.Close()

	var stats []TeacherStats
	for rows.Next() {
		var s TeacherStats
		err := rows.Scan(&s.TeacherID, &s.TeacherName, &s.AvgGrade, &s.GradeCount, &s.CourseCount)
		if err != nil {
			return nil, fmt.Errorf("scan teacher stats: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return stats, nil
}

// ДОПОЛНИТЕЛЬНЫЕ МЕТОДЫ (ДЛЯ ПРИМЕРА)

// GetCoursesByTeacher возвращает все курсы преподавателя
func (r *TeacherRepo) GetCoursesByTeacher(ctx context.Context, teacherID int64) ([]Course, error) {
	query := `SELECT id, name, teacher_id FROM courses WHERE teacher_id = $1 ORDER BY name`
	rows, err := r.pool.Query(ctx, query, teacherID)
	if err != nil {
		return nil, fmt.Errorf("get courses: %w", err)
	}
	defer rows.Close()

	var courses []Course
	for rows.Next() {
		var c Course
		err := rows.Scan(&c.ID, &c.Name, &c.TeacherID)
		if err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		courses = append(courses, c)
	}
	return courses, nil
}

// ПРИМЕР ИСПОЛЬЗОВАНИЯ В MAIN

func main() {
	// Подключение к БД
	connString := "postgres://user:password@localhost:5432/mydb?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}
	defer pool.Close()

	repo := NewTeacherRepo(pool)
	ctx := context.Background()

	// 1. Создаём преподавателей (если их нет)
	teachers := []string{"Иван Петров", "Мария Смирнова", "Алексей Иванов"}
	for _, name := range teachers {
		_, err := repo.CreateTeacher(ctx, name)
		if err != nil {
			log.Printf("Teacher already exists or error: %v", err)
		}
	}

	// 2. Получаем топ-5 преподавателей
	stats, err := repo.GetTopTeachersByAvgGrade(ctx, 2, 5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Топ-5 преподавателей по средней оценке:")
	for i, s := range stats {
		fmt.Printf("%d. %s (ID: %d) — средняя: %.2f, оценок: %d, курсов: %d\n",
			i+1, s.TeacherName, s.TeacherID, s.AvgGrade, s.GradeCount, s.CourseCount)
	}
}
