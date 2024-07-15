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

	//go:embed assets/*
	assets embed.FS
)

func main() {
	conn, err := pgx.Connect(context.Background(), DATABASE_URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
	}
	defer conn.Close(context.Background())

	http.Handle("GET /assets/*", http.FileServerFS(assets))

	http.HandleFunc("PATCH /flashcards/{id}", func(w http.ResponseWriter, r *http.Request) {
		answer := r.FormValue("answer")
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}

		tag, err := conn.Exec(context.Background(), `UPDATE flashcards SET answer = $1 WHERE id = $2`, answer, id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}
		w.Write([]byte(fmt.Sprintf("inserted rows: %d", tag.RowsAffected())))
	})
	http.HandleFunc("POST /flashcards", func(w http.ResponseWriter, r *http.Request) {
		prompt := r.FormValue("prompt")
		answer := r.FormValue("answer")

		tag, err := conn.Exec(context.Background(), `INSERT INTO flashcards (prompt, answer) VALUES ($1,$2)`, prompt, answer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}

		w.Write([]byte(fmt.Sprintf("inserted rows: %d", tag.RowsAffected())))
	})
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(context.Background(), `select * from flashcards`)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}

		type Flashcard struct {
			ID     int64
			Prompt string
			Answer *string
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

		t, err := template.New("homepage").Parse(`
		<!DOCTYPE html>
		<html>
		  <head>
		  	<script src="https://unpkg.com/htmx.org@2.0.1" integrity="sha384-QWGpdj554B4ETpJJC9z+ZHJcA/i59TyjxEPXiiUgN2WmTyV5OEZWCD6gQhgkdpB/" crossorigin="anonymous"></script>
		  	<link rel="stylesheet" href="/assets/root.css">
		  </head>
          <body>
		    <h1>Flashcards</h1>
			<div class="flashcard" style="width: max(75%, 256px); margin-inline-start: 2rem; margin-block: 2rem">
				<div>New flashcard</div>
				<form hx-post="/flashcards">
					<label for="prompt">prompt:</label>
					<input id="prompt" name="prompt" type="text" />
					<label for="answer">answer:</label>
					<input id="answer" name="answer" type="text"/>
					<button>create</button>
				</form>
			</div>
			<div class="flashcard-container">
				{{ range $flashcard := . }}
				<div class="flashcard">
					<div>
						<div>{{ .Prompt }}</div>
						<button hx-on:click="document.getElementById('flashcard-{{.ID}}').classList.toggle('hidden')">show/hide</button>
					</div>
					<div id="flashcard-{{ .ID }}" class="flashcard-answer hidden">
						<form hx-patch="/flashcards/{{.ID}}" hx-swap="none">
							Answer: <input type="text" name="answer" value="{{ .Answer }}"></input>
							<button type="submit">update</button>
						</form>
					</div>
				</div>
				{{ end }}
			</div>
		  </body>
		</html>
		`)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}
		t.Execute(w, flashcards)
	})
	if err := http.ListenAndServe(PORT, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
