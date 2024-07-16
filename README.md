# flashcards

## development
For first time setup, you will need to initialize tables. You can do this in `psql`shell by executing `install.sql`.
```psql
flashcards=# \i install.sql;
```
Start the server. If you'd like hot-reloading, start the server with something like [air](https://github.com/air-verse/air).
```zsh
go run ./server.go
```
