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

	http.HandleFunc("GET /assets/*", func(w http.ResponseWriter, r *http.Request) {
		http.FileServerFS(assets).ServeHTTP(w, r)
	})

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

		row := conn.QueryRow(context.Background(), `INSERT INTO flashcards (prompt, answer) VALUES ($1,$2) RETURNING *`, prompt, answer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}

		var created Flashcard
		if err := row.Scan(&created.ID, &created.Prompt, &created.Answer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal server error: %s", err)
			return
		}

		t, _ := template.New("flashcard-frag").Parse(`
				<div class="flashcard">
					<div class="flashcard-prompt">
						<div>{{ .Prompt }}</div>
					</div>
					<div id="flashcard-{{ .ID }}" class="flashcard-answer">
						<form id="update-flashcard-{{.ID}}-answer" hx-patch="/flashcards/{{.ID}}" hx-swap="none" class="hidden">
							Answer: <input type="text" name="answer" value="{{ .Answer }}"></input>
							<button type="submit">update</button>
							</form>
						<button hx-on:click="document.getElementById('update-flashcard-{{.ID}}-answer').classList.toggle('hidden')">👀</button>
					</div>
				</div>`)
		t.Execute(w, created)
	})
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(context.Background(), `select * from flashcards`)
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

		t, err := template.New("homepage").Parse(`
		<!DOCTYPE html>
		<html>
		  <head>
		  	<script src="https://unpkg.com/htmx.org@2.0.1" integrity="sha384-QWGpdj554B4ETpJJC9z+ZHJcA/i59TyjxEPXiiUgN2WmTyV5OEZWCD6gQhgkdpB/" crossorigin="anonymous"></script>
		  	<link rel="stylesheet" href="/assets/root.css">
		  </head>
          <body>
		    <h1>Flashcards</h1>
			<div class="flashcard">
				<form hx-post="/flashcards" hx-swap="afterbegin" hx-target=".flashcard-container" hx-on::after-request="this.reset()">
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
					<div class="flashcard-prompt">
						<div>{{ .Prompt }}</div>
						<details>
							<summary>answer</summary>
							<p>{{.Answer}}</p>
						</details>
					</div>
					<div id="flashcard-{{ .ID }}" class="flashcard-answer">
						<form id="update-flashcard-{{.ID}}-answer" hx-patch="/flashcards/{{.ID}}" hx-swap="none" class="hidden">
							Answer: <input type="text" name="answer" value="{{ .Answer }}"></input>
							<button type="submit">update</button>
							</form>
						<button hx-on:click="document.getElementById('update-flashcard-{{.ID}}-answer').classList.toggle('hidden')">✏️</button>
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
