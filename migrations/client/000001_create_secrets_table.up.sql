CREATE TABLE IF NOT EXISTS secrets (
    id integer NOT NULL PRIMARY KEY,
    type integer NOT NULL
);

CREATE TABLE IF NOT EXISTS secrets_type (
    id integer NOT NULL PRIMARY KEY,
    type text(255) NOT NULL
);

INSERT INTO secrets_type VALUES(0, 'login_password');
INSERT INTO secrets_type VALUES(1, 'credit_card');
