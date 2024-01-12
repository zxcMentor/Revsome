package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Age      int    `json:"age"`
}

type UserRepository interface {
	CreateUser(user User) error
	GetAllUsers() ([]User, error)
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func CreateTable(db *sql.DB) error {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS usersr (
			email TEXT PRIMARY KEY,
			password TEXT,
			name TEXT,
			age INTEGER
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *UserRepo) CreateUser(user User) error {

	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM usersr
		WHERE email = ? 
	`, user.Email).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("user with the %s email already exists", user.Email)
	}

	if user.Age < 18 {
		return fmt.Errorf("user must be at least 18 years old")
	}

	_, err = s.db.Exec(`
		INSERT INTO usersr (email, password, name, age)
		VALUES (?, ?, ?, ?)
	`, user.Email, user.Password, user.Name, user.Age)

	return err
}

func (s *UserRepo) GetAllUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT email, password, name, age FROM usersr`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.Email, &user.Password, &user.Name, &user.Age)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

type UserServ interface {
	CreateUser(user User) error
	GetAllUsers() ([]User, error)
}

type UserService struct {
	RepoUser *UserRepo
}

func NewUserService(db *UserRepo) *UserService {
	return &UserService{db}
}

func (u *UserService) CreateUser(user User) error {

	return u.RepoUser.CreateUser(user)
}

func (u *UserService) GetAllUsers() ([]User, error) {
	return u.RepoUser.GetAllUsers()
}

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	GetCacheAllUsers() ([]User, error)
}

type CachedDatabase struct {
	Database *UserRepo
	cache    map[string]interface{}
	mu       sync.RWMutex
}

func NewCachedDatabase(db *UserRepo) *CachedDatabase {
	return &CachedDatabase{
		Database: db,
		cache:    make(map[string]interface{}),
	}
}

func (c *CachedDatabase) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.cache[key]
	return value, ok
}

func (c *CachedDatabase) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = value
}

func (c *CachedDatabase) GetCacheAllUsers() ([]User, error) {
	if value, ok := c.Get("all_users"); ok {
		if users, ok := value.([]User); ok {
			fmt.Println("Users from cache")
			return users, nil
		}
	}

	users, err := c.Database.GetAllUsers()
	if err != nil {
		return nil, err
	}

	c.Set("all_users", users)

	return users, nil
}

type UserHandler struct {
	UserServ *UserService
	CacheDB  *CachedDatabase
}

func NewUserHandler(uServ *UserService, cache *CachedDatabase) *UserHandler {
	return &UserHandler{uServ, cache}
}

func (h *UserHandler) CreateHand(w http.ResponseWriter, r *http.Request) {

	user := User{
		Email:    "Аpopo",
		Password: "sdgsg",
		Name:     "gsfdgsd",
		Age:      20,
	}

	if err := h.UserServ.CreateUser(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Write([]byte("Create user"))
	w.WriteHeader(http.StatusCreated)
}

func (h *UserHandler) GetHand(w http.ResponseWriter, r *http.Request) {
	users, err := h.CacheDB.GetCacheAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(users); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	r := chi.NewRouter()
	db, err := sql.Open("sqlite3", "users.db")
	if err != nil {
		log.Fatal(err)
	}
	usRep := NewUserRepo(db)
	usServ := NewUserService(usRep)
	cachedDB := NewCachedDatabase(usRep)
	usHand := NewUserHandler(usServ, cachedDB)
	err = CreateTable(db)
	if err != nil {
		log.Fatal(err)
	}

	r.Get("/create", usHand.CreateHand)

	r.Get("/users", usHand.GetHand)

	log.Fatal(http.ListenAndServe(":8080", r))
}
