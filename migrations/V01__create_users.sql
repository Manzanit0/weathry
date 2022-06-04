CREATE TABLE users(
    chat_id VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255),
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    language_code VARCHAR(255) NOT NULL DEFAULT 'en',
    is_bot BOOLEAN NOT NULL DEFAULT FALSE,

    PRIMARY KEY(chat_id)
);