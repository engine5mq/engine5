## Veritabanı crud işlemi

```go
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// User represents the database entity structure
type User struct {
	ID    int
	Name  string
	Email string
}

func main() {
	// 1. Connect to the SQLite Database (creates a file named sample.db)
	db, err := sql.Open("sqlite3", "./sample.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 2. Initialize Table
	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatal(err)
	}

	// === 3. CRUD OPERATIONS ===

	// CREATE
	userID, err := createUser(db, "Alice Smith", "alice@example.com")
	if err != nil {
		log.Fatalf("Error creating user: %v", err)
	}
	fmt.Printf("[CREATE] Successfully inserted user ID: %d\n", userID)

	// READ (Single Record)
	user, err := getUserByID(db, userID)
	if err != nil {
		log.Fatalf("Error reading user: %v", err)
	}
	fmt.Printf("[READ] Found user: ID=%d, Name=%s, Email=%s\n", user.ID, user.Name, user.Email)

	// UPDATE
	err = updateUserEmail(db, userID, "alice.new@example.com")
	if err != nil {
		log.Fatalf("Error updating user: %v", err)
	}
	fmt.Println("[UPDATE] User email modified successfully.")

	// READ (All Records)
	allUsers, err := getAllUsers(db)
	if err != nil {
		log.Fatalf("Error reading all users: %v", err)
	}
	fmt.Println("[READ ALL] Current users in database:")
	for _, u := range allUsers {
		fmt.Printf(" - ID: %d, Name: %s, Email: %s\n", u.ID, u.Name, u.Email)
	}

	// DELETE
	err = deleteUser(db, userID)
	if err != nil {
		log.Fatalf("Error deleting user: %v", err)
	}
	fmt.Printf("[DELETE] User ID %d deleted successfully.\n", userID)
}

// Create operation using prepared statements
func createUser(db *sql.DB, name, email string) (int, error) {
	query := `INSERT INTO users (name, email) VALUES (?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(name, email)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return int(id), err
}

// Read operation for a single row
func getUserByID(db *sql.DB, id int) (*User, error) {
	query := `SELECT id, name, email FROM users WHERE id = ?`
	row := db.QueryRow(query, id)

	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Read operation for multiple rows
func getAllUsers(db *sql.DB) ([]User, error) {
	query := `SELECT id, name, email FROM users`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// Update operation
func updateUserEmail(db *sql.DB, id int, newEmail string) error {
	query := `UPDATE users SET email = ? WHERE id = ?`
	_, err := db.Exec(query, newEmail, id)
	return err
}

// Delete operation
func deleteUser(db *sql.DB, id int) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}

```