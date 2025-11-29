CREATE TABLE IF NOT EXISTS deleted (
    secret_id integer NOT NULL,
    id integer NOT NULL,
    type_id integer(1) NOT NULL,
    version integer NOT NULL
);

CREATE TABLE IF NOT EXISTS deleted_type (
    id integer NOT NULL PRIMARY KEY,
    type text(255) NOT NULL
);

INSERT INTO deleted_type (id, type) VALUES (1, 'meta');
INSERT INTO deleted_type (id, type) VALUES (2, 'data');