CREATE TABLE IF NOT EXISTS deleted (
    secret_id integer NOT NULL PRIMARY KEY,
    locked_at integer
);