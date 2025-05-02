package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"go-rest-api/internal/models"
	"go-rest-api/internal/repository/sqlconnect"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var (
	teachers = make(map[int]models.Teacher)
	nextID   = 1
)

func isValidSortOrder(order string) bool {
	return order == "asc" || order == "desc"
}

func isValidSortField(field string) bool {
	validFields := map[string]bool{
		"first_name": true,
		"last_name":  true,
		"email":      true,
		"class":      true,
		"subject":    true,
	}

	return validFields[field]
}

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
		updateTeacherHandler(w, r)
	case http.MethodPatch:
		PatchTeacherHandler(w, r)
	case http.MethodDelete:
		DeleteTeacherHandler(w, r)
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

		query := "SELECT id, first_name, last_name, email, class, subject FROM teachers WHERE 1=1"
		queryCount := "SELECT COUNT(id) FROM teachers WHERE 1=1"
		var args []interface{}
		var argsCount []interface{}

		query, args, queryCount, argsCount = addFilters(r, query, queryCount, args, argsCount)

		query = addSorting(r, query)

		// Add pagination
		offset := (page - 1) * limit
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err := db.Query(query, args...)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Database query error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		teacherList := make([]models.Teacher, 0)
		for rows.Next() {
			var teacher models.Teacher
			err := rows.Scan(&teacher.ID, &teacher.FirstName, &teacher.LastName, &teacher.Email, &teacher.Class, &teacher.Subject)
			if err != nil {
				http.Error(w, "Database scan error", http.StatusInternalServerError)
				return
			}
			teacherList = append(teacherList, teacher)
		}

		var totalTeachers int

		err = db.QueryRow(queryCount, argsCount...).Scan(&totalTeachers)
		if err != nil {
			fmt.Println(err)
			totalTeachers = 0
		}

		response := struct {
			Status string           `json:"status"`
			Count  int              `json:"count"`
			Data   []models.Teacher `json:"data"`
		}{
			Status: "success",
			Count:  totalTeachers,
			Data:   teacherList,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	var teacher models.Teacher

	query := "SELECT id, first_name, last_name, email, class, subject FROM teachers WHERE id = ?"
	row := db.QueryRow(query, idStr)

	err = row.Scan(&teacher.ID, &teacher.FirstName, &teacher.LastName, &teacher.Email, &teacher.Class, &teacher.Subject)
	if err == sql.ErrNoRows {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	} else if err != nil {
		fmt.Println(err)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teacher)
}

func addSorting(r *http.Request, query string) string {
	// teachers/?sort_by=name:asc&sort_by=class:desc
	sortParams := r.URL.Query()["sort_by"] // slice of strings
	if len(sortParams) > 0 {
		query += " ORDER BY"
		for i, param := range sortParams {
			parts := strings.Split(param, ":")
			if len(parts) != 2 {
				continue
			}
			field, order := parts[0], parts[1]
			if !isValidSortField(field) || !isValidSortOrder(order) {
				continue
			}
			if i > 0 {
				query += ","
			}
			query += " " + field + " " + order
		}
	}

	return query
}

func addFilters(r *http.Request, query, queryCount string, args, argsCount []interface{}) (string, []interface{}, string, []interface{}) {
	params := map[string]string{
		"first_name": "first_name",
		"last_name":  "last_name",
		"email":      "email",
		"class":      "class",
		"subject":    "subject",
	}

	for param, dbField := range params {
		value := r.URL.Query().Get(param)
		if value != "" {
			query += " AND " + dbField + " = ?"
			queryCount += " AND " + dbField + " = ?"
			args = append(args, value)
			argsCount = append(argsCount, value)
		}
	}

	return query, args, queryCount, argsCount
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

func updateTeacherHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/teachers/")

	var updatedTeacher models.Teacher
	err := json.NewDecoder(r.Body).Decode(&updatedTeacher)
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid Request Payload", http.StatusBadRequest)
		return
	}

	db, err := sqlconnect.ConnectDb()
	if err != nil {
		log.Println(err)
		http.Error(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var existingTeacher models.Teacher
	query := "SELECT id, first_name, last_name, email, class, subject FROM teachers WHERE id = ?"
	err = db.QueryRow(query, idStr).Scan(
		&existingTeacher.ID,
		&existingTeacher.FirstName,
		&existingTeacher.LastName,
		&existingTeacher.Email,
		&existingTeacher.Class,
		&existingTeacher.Subject,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Unable to retrieve data", http.StatusNotFound)
	}

	updatedTeacher.ID = existingTeacher.ID
	updateQuery := `
	UPDATE teachers 
	SET first_name = ?, last_name = ?, email = ?, class = ?, subject = ? 
	WHERE id = ?
`
	_, err = db.Exec(
		updateQuery,
		updatedTeacher.FirstName,
		updatedTeacher.LastName,
		updatedTeacher.Email,
		updatedTeacher.Class,
		updatedTeacher.Subject,
		updatedTeacher.ID,
	)
	if err != nil {
		http.Error(w, "Unable to update data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedTeacher)
}

func PatchTeacherHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/teachers/")

	var updates map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid Request Payload", http.StatusBadRequest)
		return
	}

	db, err := sqlconnect.ConnectDb()
	if err != nil {
		log.Println(err)
		http.Error(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var existingTeacher models.Teacher
	query := `
		SELECT id, first_name, last_name, email, class, subject 
		FROM teachers 
		WHERE id = ?
		`
	err = db.QueryRow(query, idStr).Scan(
		&existingTeacher.ID,
		&existingTeacher.FirstName,
		&existingTeacher.LastName,
		&existingTeacher.Email,
		&existingTeacher.Class,
		&existingTeacher.Subject,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Teacher not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Unable to retrieve data", http.StatusNotFound)
	}

	// Apply updates (NOT IN USE IN REAL WORLD)
	//for k, v := range updates {
	//	switch strings.ToLower(k) {
	//	case "first_name":
	//		existingTeacher.FirstName = v.(string)
	//	case "last_name":
	//		existingTeacher.LastName = v.(string)
	//	case "email":
	//		existingTeacher.Email = v.(string)
	//	case "class":
	//		existingTeacher.Class = v.(string)
	//	case "subject":
	//		existingTeacher.Subject = v.(string)
	//	}
	//}

	// Apply updates using reflect
	teacherVal := reflect.ValueOf(&existingTeacher).Elem()
	//fmt.Println("teacherVal:", teacherVal)
	//fmt.Println("teacherVal Type:", teacherVal.Type())
	//fmt.Println("teacherVal Field 0:", teacherVal.Type().Field(0))
	//fmt.Println("teacherVal Field 1:", teacherVal.Type().Field(1))
	//fmt.Println("teacherVal Field 2:", teacherVal.Type().Field(2))
	teacherType := teacherVal.Type()

	for k, v := range updates {
		for i := 0; i < teacherVal.NumField(); i++ {
			field := teacherType.Field(i)
			//fmt.Println(field.Tag.Get("json"))
			if field.Tag.Get("json") == k+",omitempty" {
				if teacherVal.Field(i).CanSet() {
					fieldVal := teacherVal.Field(i)
					//fmt.Println("fieldVal", fieldVal)
					//fmt.Println("reflect.ValueOf(v)", reflect.ValueOf(v))
					//fmt.Println("teacherVal.Field(i).Type()", teacherVal.Field(i).Type())
					fieldVal.Set(reflect.ValueOf(v).Convert(teacherVal.Field(i).Type()))
				}
			}
		}
	}

	query = `
		UPDATE teachers
		SET first_name = ?, last_name = ?, email = ?, class = ?, subject = ?
		WHERE id = ?
		`
	_, err = db.Exec(
		query,
		existingTeacher.FirstName,
		existingTeacher.LastName,
		existingTeacher.Email,
		existingTeacher.Class,
		existingTeacher.Subject,
		idStr,
	)
	if err != nil {
		http.Error(w, "Unable to update data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existingTeacher)
}

func DeleteTeacherHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/teachers/")

	db, err := sqlconnect.ConnectDb()
	if err != nil {
		log.Println(err)
		http.Error(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	result, err := db.Exec("DELETE FROM teachers WHERE id = ?", idStr)
	if err != nil {
		http.Error(w, "Unable to delete data", http.StatusInternalServerError)
		return
	}

	//fmt.Println(result.RowsAffected())
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Unable to get rows affected", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Teacher not found", http.StatusNotFound)
		return
	}

	//w.WriteHeader(http.StatusNoContent)
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}{
		Status: "success",
		ID:     idStr,
	}
	json.NewEncoder(w).Encode(response)
}
