-- Create enum types
CREATE TYPE gender AS ENUM ('male', 'female', 'other');
CREATE SEQUENCE user_external_id_seq START WITH 1;
CREATE SEQUENCE admin_external_id_seq START WITH 1;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    user_login VARCHAR(35),
    birthday DATE,
    gender gender,
    fullname VARCHAR(55),
    email VARCHAR(35) UNIQUE NOT NULL,
    phone VARCHAR(35),
    user_password VARCHAR,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS administrators (
    id UUID PRIMARY KEY,
    user_login VARCHAR,
    birthday DATE,
    gender gender,
    fullname VARCHAR(55),
    email VARCHAR(35),
    phone VARCHAR(35),
    user_password VARCHAR,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);
