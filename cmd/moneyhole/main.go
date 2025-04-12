package main

import (
	"MoneyHole/internal/aggregator"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)


func main() {
	e := echo.New()
	err := ConnectToDB()
	if err != nil {
		slog.Error(err.Error())
	}

	e.GET("/", hello)
	e.POST("/users/add", add_user)
	e.GET("/users/show", show_users)

	go func() {
		if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start server", "error", err)
		}
	}()
	for {
		go aggregator.Run()
		time.Sleep(10 * time.Minute)
	}
}


func add_user(c echo.Context) error {
	user := new(User)

	if err := c.Bind(user); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON format",
		})
	}
	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`Insert into users values($1, $2, $3)`, &user.Id, &user.Name, &user.Id_message)
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, "new user was create")
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "hello, world!")
}

func show_users(c echo.Context) error {
	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`select * from users`)
	if err != nil {
		return err
	}
	defer rows.Close()

	res := make([]User, 0)
	for rows.Next() {
		var id, id_m int
		var s string
		err = rows.Scan(&id, &s, &id_m) 
		if err != nil {
            return err
        }
		res = append(res, User{id, s, id_m})
	}
	return c.JSON(http.StatusOK, res)
	
}

func ConnectToDB() error {
	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE if not exists users (
		id INTEGER Primiry Key,
		name varchar(50),
		id_message INTEGER
	)`)
	if err != nil {
        return err
    }
	return nil
}

type User struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Id_message int `json:"id_message"`
}