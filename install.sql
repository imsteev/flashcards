create table if not exists flashcards (
	id SERIAL PRIMARY KEY,
	prompt TEXT NOT NULL,
	answer TEXT -- this can be null in case you're still researching the answer
);

create table if not exists flashcard_tags (
	id SERIAL PRIMARY KEY,
	flashcard_id INTEGER NOT NULL,
	tag VARCHAR(100) NOT NULL UNIQUE
);
