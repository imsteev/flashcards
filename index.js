import postgres from 'postgres';

const sql = postgres({ database: 'flashcards' });

async function getFlashcards() {
	return sql`select * from flashcards`;
}

async function insertFlashcard(prmpt) {
	return sql`insert into flashcards (prompt) values (${prmpt})`
}	

await insertFlashcard('how big is a football stadium');

console.log(await getFlashcards());


// this is needed otherwise script can't exist.
// there must be some before-exit-hook that postgres.js registers...(blegh)
await sql.end({ timeout: 5 });

