package main

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
)

var (
	PORT         = cmp.Or(os.Getenv("PORT"), ":8080")
	DATABASE_URL = cmp.Or(os.Getenv("DATABASE_URL"), "postgres://localhost:5432/flashcards")

	//go:embed assets
	assets embed.FS
	//go:embed views
	views embed.FS
)

type Flashcard struct {
	ID     int64
	Prompt string
	Answer *string
}

func main() {
	conn, err := pgx.Connect(context.Background(), DATABASE_URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
	}
	defer conn.Close(context.Background())
	app := &App{conn}
	http.HandleFunc("GET /assets/*", app.ServeAssets)
	http.HandleFunc("PATCH /flashcards/{id}", app.PatchFlashcard)
	http.HandleFunc("POST /flashcards", app.CreateFlashcard)
	http.HandleFunc("GET /", app.GetHomePage)
	if err := http.ListenAndServe(PORT, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

type App struct {
	// NOT CONCURRENT SAFE (yet)
	conn *pgx.Conn
}

func (a *App) ServeAssets(w http.ResponseWriter, r *http.Request) {
	http.FileServerFS(assets).ServeHTTP(w, r)
}
func (a *App) PatchFlashcard(w http.ResponseWriter, r *http.Request) {
	answer := r.FormValue("answer")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}

	tag, err := a.conn.Exec(context.Background(), `UPDATE flashcards SET answer = $1 WHERE id = $2`, answer, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}
	w.Write([]byte(fmt.Sprintf("inserted rows: %d", tag.RowsAffected())))
}

func (a *App) GetHomePage(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(context.Background(), `select * from flashcards`)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}

	var flashcards []Flashcard
	for rows.Next() {
		var f Flashcard
		if err := rows.Scan(&f.ID, &f.Prompt, &f.Answer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}
		flashcards = append(flashcards, f)
	}

	// interpolate the flashcards in a list

	tmpls, err := template.ParseFS(views, "views/root.html", "views/flashcard.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}
	tmpls.ExecuteTemplate(w, "root", flashcards)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}
}

func (a *App) CreateFlashcard(w http.ResponseWriter, r *http.Request) {
	prompt := r.FormValue("prompt")
	answer := r.FormValue("answer")

	row := a.conn.QueryRow(context.Background(), `INSERT INTO flashcards (prompt, answer) VALUES ($1,$2) RETURNING *`, prompt, answer)

	var created Flashcard
	if err := row.Scan(&created.ID, &created.Prompt, &created.Answer); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error: %s", err)
		return
	}

	t, _ := template.ParseFS(views, "flashcard")
	t.Execute(w, created)
}
