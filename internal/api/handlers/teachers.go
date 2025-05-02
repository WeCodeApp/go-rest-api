package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"go-rest-api/internal/models"
	"go-rest-api/internal/repository/sqlconnect"
	"net/http"
	"strconv"
	"strings"
)

var (
	teachers = make(map[int]models.Teacher)
	nextID   = 1
)

func init() {
	teachers[nextID] = models.Teacher{
		ID:        "1",
		FirstName: "Juan",
		LastName:  "Perez",
		Class:     "Apo",
		Subject:   "Math",
	}
	nextID++
	teachers[nextID] = models.Teacher{
		ID:        "2",
		FirstName: "Pedro",
		LastName:  "Castro",
		Class:     "Rizal",
		Subject:   "Science",
	}
	nextID++
}

func TeachersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTeachersHandler(w, r)
	case http.MethodPost:
		addTeachersHandler(w, r)
	case http.MethodPut:
		w.Write([]byte("Hello PUT teachers Route"))
		fmt.Println("Hello PUT teachers Route")
	case http.MethodPatch:
		w.Write([]byte("Hello PATCH teachers Route"))
		fmt.Println("Hello PATCH teachers Route")
	case http.MethodDelete:
		w.Write([]byte("Hello DELETE teachers Route"))
		fmt.Println("Hello DELETE teachers Route")
	}
}

func getTeachersHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sqlconnect.ConnectDb()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	path := strings.TrimPrefix(r.URL.Path, "/teachers/")
	idStr := strings.TrimSuffix(path, "/")

	if idStr == "" {
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")

		if pageStr == "" {
			pageStr = "1"
		}
		if limitStr == "" {
			limitStr = "10"
		}

		page, _ := strconv.Atoi(pageStr)
		limit, _ := strconv.Atoi(limitStr)

		maxLimit := 100
		if limit > maxLimit {
			http.Error(w, "Limit cannot be greater than 100", http.StatusBadRequest)
			return
		}

		query := "SELECT id, first_name, last_name, subject, class, email FROM teachers WHERE 1=1"
		queryTotal := "SELECT COUNT(id) FROM teachers WHERE 1=1 "
		var args []interface{}
		var argsTotal []interface{}

		params := map[string]string{
			"first_name": "first_name",
			"last_name":  "last_name",
			"email":      "email",
			"class":      "class",
			"subject":    "subject",
		}

		for param, dbField := range params {
			if value := r.URL.Query().Get(param); value != "" {
				query += " AND " + dbField + " = ? "
				queryTotal += " AND " + dbField + " = ? "
				args = append(args, value)
				argsTotal = append(argsTotal, value)
			}
		}

		offset := (page - 1) * limit
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		teacherList := make([]models.Teacher, 0)

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Error querying teachers", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var teacher models.Teacher
			err = rows.Scan(&teacher.ID, &teacher.FirstName, &teacher.LastName, &teacher.Subject, &teacher.Class, &teacher.Email)
			if err != nil {
				http.Error(w, "Error scanning teachers", http.StatusInternalServerError)
				return
			}
			teacherList = append(teacherList, teacher)
		}

		var total int
		err = db.QueryRow(queryTotal, argsTotal...).Scan(&total)
		if err != nil {
			http.Error(w, "Error scanning total teachers", http.StatusInternalServerError)
			return
		}

		response := struct {
			Status string           `json:"status"`
			Count  int              `json:"count"`
			Data   []models.Teacher `json:"data"`
		}{
			Status: "success",
			Count:  total,
			Data:   teacherList,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Println(err)
		return
	}

	teacher, exists := teachers[id]
	if !exists {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(teacher)
}

func addTeachersHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sqlconnect.ConnectDb()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	var newTeachers []models.Teacher
	err = json.NewDecoder(r.Body).Decode(&newTeachers)
	if err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare(`
		INSERT INTO teachers (id, first_name, last_name, subject, class, email)
		VALUES (?,?, ?, ?, ?, ?)
	`)
	if err != nil {
		http.Error(w, "Error preparing statement", http.StatusInternalServerError)
	}
	defer stmt.Close()

	addedTeachers := make([]models.Teacher, 0, len(newTeachers))
	for _, newTeacher := range newTeachers {
		//check for duplicate email
		var existingID string
		err = db.QueryRow("SELECT id FROM teachers WHERE email = ?", newTeacher.Email).Scan(&existingID)
		if err == nil {
			//Email already exists
			http.Error(w, "Email already exists", http.StatusBadRequest)
			return
		}

		id := uuid.New().String()
		_, err := stmt.Exec(id, newTeacher.FirstName, newTeacher.LastName, newTeacher.Subject, newTeacher.Class, newTeacher.Email)
		if err != nil {
			http.Error(w, "Error inserting teacher", http.StatusInternalServerError)
			return
		}
		newTeacher.ID = id
		addedTeachers = append(addedTeachers, newTeacher)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := struct {
		Status string           `json:"status"`
		Count  int              `json:"count"`
		Data   []models.Teacher `json:"data"`
	}{
		Status: "success",
		Count:  len(addedTeachers),
		Data:   addedTeachers,
	}

	json.NewEncoder(w).Encode(response)
}
