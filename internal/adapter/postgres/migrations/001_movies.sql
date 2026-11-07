CREATE TABLE movies (
    id BIGINT PRIMARY KEY CHECK (id > 0),
    title TEXT NOT NULL,
    year INTEGER NOT NULL,
    director TEXT NOT NULL
);
