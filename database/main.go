package database

import (
	"database/sql"
	"fmt"
	"time"

	"hacknhbackend.eparker.dev/courseload"
	_ "modernc.org/sqlite"
)

const COURSES_STATEMENT = `CREATE TABLE IF NOT EXISTS courses (
    term_crn TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    subject_code TEXT NOT NULL,
    course_number TEXT NOT NULL,
    description TEXT NOT NULL
);`

const INSTRUCTORS_STATEMENT = `CREATE TABLE IF NOT EXISTS instructors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    last_name TEXT NOT NULL,
    first_name TEXT NOT NULL,
    email TEXT NOT NULL,
    term_crn TEXT NOT NULL,
    FOREIGN KEY (term_crn) REFERENCES courses(term_crn)
);`

const MEETINGS_STATEMENT = `CREATE TABLE IF NOT EXISTS meetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    days TEXT NOT NULL,
    building TEXT NOT NULL,
    room TEXT NOT NULL,
    time TEXT NOT NULL,
    term_crn TEXT NOT NULL,
    FOREIGN KEY (term_crn) REFERENCES courses(term_crn)
);`

const USERS_STATEMENT = `CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    classes TEXT NOT NULL
);`

const INSERT_USER_STATEMENT = `INSERT INTO users (username, password, classes) VALUES (?, ?, ?);`
const INSERT_INSTUCTOR_STATEMENT = `INSERT INTO instructors (last_name, first_name, email, term_crn) VALUES (?, ?, ?, ?);`
const INSERT_MEETING_STATEMENT = `INSERT INTO meetings (days, building, room, time, term_crn) VALUES (?, ?, ?, ?, ?);`
const INSERT_COURSE_STATEMENT = `INSERT INTO courses (term_crn, title, subject_code, course_number, description) VALUES (?, ?, ?, ?, ?);`

const SELECT_USER_STATEMENT = `SELECT id, username, password, classes FROM users WHERE username = ?;`
const SELECT_COUSE_STATEMENT = `SELECT term_crn, title, subject_code, course_number, description FROM courses WHERE term_crn = ?;`
const SELECT_INSTRUCTORS_STATEMENT = `SELECT id, last_name, first_name, email FROM instructors WHERE term_crn = ?;`
const SELECT_MEETINGS_STATEMENT = `SELECT id, days, building, room, time FROM meetings WHERE term_crn = ?;`

const (
	maxRetries = 5
	baseDelay  = 100 * time.Millisecond
)

var db *sql.DB

func OpenDatabase() (*sql.DB, error) {
	var err error

	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("sqlite", "hacknh.db")
		if err == nil {
			break
		}

		time.Sleep(baseDelay * time.Duration(i))
	}

	return db, err
}

func InsertCourse(course courseload.Course) error {
	_, err := db.Exec(INSERT_COURSE_STATEMENT, course.CRN, course.Data.Title, course.Data.Subject, course.Data.Number, course.Data.Description)
	if err != nil {
		return err
	}

	for _, instructor := range course.Data.Instructors {
		_, err := db.Exec(INSERT_INSTUCTOR_STATEMENT, instructor.LastName, instructor.FirstName, instructor.Email, course.CRN)
		if err != nil {
			return err
		}
	}

	for _, meeting := range course.Data.Meetings {
		_, err := db.Exec(INSERT_MEETING_STATEMENT, meeting.Days, meeting.Building, meeting.Room, meeting.Time, course.CRN)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteCourse(term_crn string) error {
	_, err := db.Exec("DELETE FROM courses WHERE term_crn = ?;", term_crn)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM instructors WHERE term_crn = ?;", term_crn)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM meetings WHERE term_crn = ?;", term_crn)
	if err != nil {
		return err
	}

	return nil
}

func GetCourse(term_crn string) (*courseload.Course, error) {
	row := db.QueryRow(SELECT_COUSE_STATEMENT, term_crn)

	var title, subject_code, course_number, description string
	err := row.Scan(&term_crn, &title, &subject_code, &course_number, &description)
	if err != nil {
		return nil, err
	}

	instructors := make([]courseload.Instructor, 0)
	rows, err := db.Query(SELECT_INSTRUCTORS_STATEMENT, term_crn)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id int
		var last_name, first_name, email string
		err = rows.Scan(&id, &last_name, &first_name, &email)
		if err != nil {
			return nil, err
		}

		instructors = append(instructors, courseload.Instructor{
			LastName:  last_name,
			FirstName: first_name,
			Email:     email,
		})
	}

	meetings := make([]courseload.Meeting, 0)
	rows, err = db.Query(SELECT_MEETINGS_STATEMENT, term_crn)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id int
		var days, building, room, time string
		err = rows.Scan(&id, &days, &building, &room, &time)
		if err != nil {
			return nil, err
		}

		meetings = append(meetings, courseload.Meeting{
			Days:     days,
			Building: building,
			Room:     room,
			Time:     time,
		})
	}

	return &courseload.Course{
		CRN: term_crn,
		Data: courseload.CourseData{
			Title:       title,
			Subject:     subject_code,
			Number:      course_number,
			Description: description,
			Instructors: instructors,
			Meetings:    meetings,
		},
	}, nil
}

func GetCourseCRNs() ([]string, error) {
	rows, err := db.Query("SELECT term_crn FROM courses;")
	if err != nil {
		return nil, err
	}

	courses := make([]string, 0)

	for rows.Next() {
		var term_crn string
		err = rows.Scan(&term_crn)
		if err != nil {
			return nil, err
		}

		courses = append(courses, term_crn)
	}

	return courses, nil
}

var QueryableKeys = map[string]string{
	"term_crn":       "CRN",
	"title":          "Title",
	"subject_code":   "Subject",
	"course_number":  "Number",
	"subject-number": "Subject & Number",
}

func QueryCourse(key string, values ...string) ([]courseload.Course, error) {
	if _, ok := QueryableKeys[key]; !ok {
		return nil, fmt.Errorf("key %s is not queryable", key)
	}

	var rows *sql.Rows
	var err error

	switch key {
	case "title":
		rows, err = db.Query("SELECT term_crn FROM courses WHERE title LIKE ?", "%"+values[0]+"%")
	case "subject-number":
		rows, err = db.Query("SELECT term_crn FROM courses WHERE subject_code = ? AND course_number LIKE ?", values[0], "%"+values[1]+"%")
	default:
		rows, err = db.Query("SELECT term_crn FROM courses WHERE "+key+" = ?", values[0])
	}

	if err != nil {
		return nil, err
	}

	courses := make([]courseload.Course, 0)

	for rows.Next() {
		var term_crn string
		err = rows.Scan(&term_crn)
		if err != nil {
			return nil, err
		}

		course, err := GetCourse(term_crn)
		if err != nil {
			return nil, err
		}

		courses = append(courses, *course)
	}

	return courses, nil
}

func Init() {
	db, err := OpenDatabase()
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(USERS_STATEMENT)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(COURSES_STATEMENT)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(INSTRUCTORS_STATEMENT)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(MEETINGS_STATEMENT)
	if err != nil {
		panic(err)
	}
}
