package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Student struct represents a student model
type Student struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

var (
    students = make(map[int]Student) // In-memory data storage
    mu       sync.Mutex              // Mutex to handle concurrent access
)

// Generate ID for new students
func generateID() int {
    rand.Seed(time.Now().UnixNano())
    return rand.Intn(10000)
}

// CreateStudent handles POST /students to create a new student
func CreateStudent(w http.ResponseWriter, r *http.Request) {
    var student Student
    if err := json.NewDecoder(r.Body).Decode(&student); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }

    mu.Lock()
    student.ID = generateID()
    students[student.ID] = student
    mu.Unlock()

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(student)
}

// GetStudents handles GET /students to retrieve all students
func GetStudents(w http.ResponseWriter, r *http.Request) {
    mu.Lock()
    defer mu.Unlock()

    var studentList []Student
    for _, student := range students {
        studentList = append(studentList, student)
    }
    json.NewEncoder(w).Encode(studentList)
}

// GetStudentByID handles GET /students/{id} to retrieve a student by ID
func GetStudentByID(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(mux.Vars(r)["id"])
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    mu.Lock()
    student, exists := students[id]
    mu.Unlock()

    if !exists {
        http.Error(w, "Student not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(student)
}

// UpdateStudentByID handles PUT /students/{id} to update a student by ID
func UpdateStudentByID(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(mux.Vars(r)["id"])
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    var updatedStudent Student
    if err := json.NewDecoder(r.Body).Decode(&updatedStudent); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }

    mu.Lock()
    if _, exists := students[id]; !exists {
        mu.Unlock()
        http.Error(w, "Student not found", http.StatusNotFound)
        return
    }
    updatedStudent.ID = id
    students[id] = updatedStudent
    mu.Unlock()

    json.NewEncoder(w).Encode(updatedStudent)
}

// DeleteStudentByID handles DELETE /students/{id} to delete a student by ID
func DeleteStudentByID(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(mux.Vars(r)["id"])
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    mu.Lock()
    if _, exists := students[id]; !exists {
        mu.Unlock()
        http.Error(w, "Student not found", http.StatusNotFound)
        return
    }
    delete(students, id)
    mu.Unlock()

    w.WriteHeader(http.StatusNoContent)
}

// GetStudentSummary generates a summary using the Ollama API
func GetStudentSummary(w http.ResponseWriter, r *http.Request) {
	log.Println("GetStudentSummary called") 
    id, err := strconv.Atoi(mux.Vars(r)["id"])
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    mu.Lock()
    student, exists := students[id]
    mu.Unlock()

    if !exists {
        http.Error(w, "Student not found", http.StatusNotFound)
        return
    }

    summary, err := callOllamaAPI(student)
    if err != nil {
        http.Error(w, "Failed to generate summary", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{"summary": summary})
}

// callOllamaAPI makes a call to the Ollama API to generate an AI-based summary
func callOllamaAPI(student Student) (string, error) {
    const ollamaURL = "http://localhost:11434/api/generate"

    prompt := fmt.Sprintf("Provide some made up information about the student with ID %d. The student's name is %s, they are %d years old, and their email is %s.Summarize the above given details in a paragraph.", student.ID, student.Name, student.Age, student.Email)


    // Prepare the request payload
    requestPayload := map[string]string{
        "model":  "llama3.2",
        "prompt": prompt,
    }

    requestBody, err := json.Marshal(requestPayload)
    if err != nil {
        return "", fmt.Errorf("Failed to encode request payload: %v", err)
    }

    req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(requestBody))
    if err != nil {
        return "", fmt.Errorf("Failed to create request: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("Failed to reach Ollama API: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("Ollama API returned non-200 status: %d", resp.StatusCode)
    }

    var summary bytes.Buffer
    decoder := json.NewDecoder(resp.Body)

    for decoder.More() {
        var chunk map[string]interface{}
        if err := decoder.Decode(&chunk); err != nil {
            return "", fmt.Errorf("Failed to decode chunk: %v", err)
        }

        // Append the response text
        if response, ok := chunk["response"].(string); ok {
            summary.WriteString(response)
        }

        // Check for the "done" flag to stop reading
        if done, ok := chunk["done"].(bool); ok && done {
            break
        }
    }

    log.Println("Generated Summary:", summary.String())
    return summary.String(), nil
}




func main() {
    r := mux.NewRouter()
    r.HandleFunc("/students", CreateStudent).Methods("POST")
    r.HandleFunc("/students", GetStudents).Methods("GET")
    r.HandleFunc("/students/{id}", GetStudentByID).Methods("GET")
    r.HandleFunc("/students/{id}", UpdateStudentByID).Methods("PUT")
    r.HandleFunc("/students/{id}", DeleteStudentByID).Methods("DELETE")
    r.HandleFunc("/students/{id}/summary", GetStudentSummary).Methods("GET")

    log.Println("API is running on port 8080...")
    log.Fatal(http.ListenAndServe(":8080", r))
}
