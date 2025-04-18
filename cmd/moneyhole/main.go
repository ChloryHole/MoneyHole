package main

import (
	"MoneyHole/internal/aggregator"
	"context"
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
	signalChan := make(chan struct{}, 1)

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("signalChan", signalChan)
			return next(c)
		}
	})

	e.GET("/", hello)
	e.POST("/users/add", add_user)
	e.POST("/headline/add", add_headline)
	e.GET("/users/show", get_users)
	e.GET("/users/quotes", get_quotes)
	e.GET("/headlines", get_headlines)

	go func() {
		if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start server", "error", err)
		}
	}()
	
	for {
		go aggregator.Run()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
		select {
		case <- ctx.Done():
		case <- signalChan:
		}
		cancel()
	}
}

func get_headlines(c echo.Context) error {
	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		return err
	}
	defer db.Close()


	users_db, err := db.Query(
		`select id, id_message from users`,
	)
	if err != nil {
		return err
	}
	defer users_db.Close()
	response := make([]UserHeadline, 0)
	for users_db.Next() {
		var id, id_message int
		err := users_db.Scan(&id, &id_message)
		if err != nil {
			return err
		}
		response = append(response, UserHeadline{
			Id: id,
			Id_message: id_message,
			Quotes: nil,
		})
	}

	// возьми список юзеров
	for i, user := range response {
		rows, err := db.Query(
			`with filter as
				(select * from users_quotes where id_user = $1)
			select quotes.name, quotes.cost
				from filter 
				left join quotes 
					on filter.id_quote = quotes.id`,
			&user.Id,		
		)
		if err != nil {
			return err
		}
	
		res := make([]Quote, 0)
		for rows.Next() {
			var cost float32
			var name string
			err = rows.Scan(&name, &cost) 
			if err != nil {
				return err
			}
			res = append(res, Quote{name, cost})
		}
		response[i].Quotes = res
		rows.Close()
	}

	// для каждого собери данные
	return c.JSON(http.StatusOK, response)
	// верни данные
}

func add_headline(c echo.Context) error {
	headline := new(Headline)

	if err := c.Bind(headline); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON format",
		})
	}
	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		return err
	}
	defer db.Close()

	ans, err := db.Query(
		`select id from quotes where id = $1`, 
		&headline.Id, 
	)
	if err != nil {
		return err
	}
	defer ans.Close()
	
	if !ans.Next() {
		_, err = db.Exec(
			`Insert into quotes values($1, $2, $3)`, 
			&headline.Id, 
			"",
			0,
		)
		if err != nil {
			return err
		}
	}
	_, err = db.Exec(
		`Insert into users_quotes values($1, $2)`, 
		&headline.Id, 
		&headline.Id_user,
	)
	if err != nil {
		return err
	}

	ch, ok := c.Get("signalChan").(chan struct{})
	if !ok {
		return c.String(http.StatusInternalServerError, "Error: chan not found")
	}
	select {
		case ch <- struct{}{}:
		default:		
	}
	
	return c.String(http.StatusOK, "new headline added")
}


// unused
func get_quotes(c echo.Context) error {
	user := new(UserId)

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

	rows, err := db.Query(
		`with filter as
			(select * from users_quotes where id_user = $1)
		select quotes.name, quotes.cost
			from filter 
			left join quotes 
				on filter.id_quote = quotes.id`,
		&user.Id,		
	)
	if err != nil {
		return err
	}

	res := make([]Quote, 0)
	for rows.Next() {
		var cost float32
		var name string
		err = rows.Scan(&name, &cost) 
		if err != nil {
            return err
        }
		res = append(res, Quote{name, cost})
	}
	rows.Close()

	rows, err = db.Query(
		`select id_message from users where id = $1`, 
		&user.Id,
	)
	if err != nil {
		return err
	}
	var id int
	if rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return err
		}
	}
	rows.Close()
	response := UserHeadline{
		Quotes: res,
		Id: user.Id,
		Id_message: id,
	}
	return c.JSON(http.StatusOK, response)

	
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

	return c.String(http.StatusOK, "new user created")
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "hello, world!")
}

func get_users(c echo.Context) error {
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
		id INTEGER Primary Key,
		name varchar(50),
		id_message INTEGER
	)`)
	if err != nil {
        return err
    }
	_, err = db.Exec(`
	CREATE TABLE if not exists users_quotes (
		id_quote varchar(12),
		id_user INTEGER
	)`)
	if err != nil {
        return err
    }

	_, err = db.Exec(`
	CREATE TABLE if not exists quotes (
		id varchar(12),
		name varchar(50),
		cost REAL
	)`)
	if err != nil {
        return err
    }

	return nil
}

type Headline struct {
	Id string `json:"id"`
	Id_user int `json:"id_user"`
}

type User struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Id_message int `json:"id_message"`
}

type UserId struct {
	Id int `json:"id"`
}

type UserHeadline struct{
	Id int `json:"id"`
	Id_message int `json:"id_message"`
	Quotes []Quote `json:"quotes"`
}

type Quote struct {
	Name string `json:"name"`
	Cost float32 `json:"cost"`
}





