BEGIN;

CREATE EXTENSION citext;

CREATE TABLE locations (
    name CITEXT NOT NULL,

    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,

    country CITEXT,
    country_code CITEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (name)
);

CREATE TRIGGER locations
BEFORE UPDATE ON locations
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();

CREATE TABLE user_locations (
    location_name CITEXT NOT NULL,
    user_id TEXT NOT NULL,
    is_home BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (location_name, user_id),
    FOREIGN KEY (location_name) REFERENCES locations (name),
    FOREIGN KEY (user_id) REFERENCES users (chat_id)
);

-- Notes: https://stackoverflow.com/questions/16236365
-- 1. this won't allow creating FKs referencig that partially unique field.
-- 2. this index effects can't be deferred.
--
-- NB(manzanit0): I decided to take this approach regardless because it won't
-- be typical for a user to have many locations. The data integrity is nice.
CREATE UNIQUE INDEX only_one_home_per_user
ON user_locations (user_id)
WHERE (is_home = TRUE);


CREATE TRIGGER user_locations
BEFORE UPDATE ON user_locations
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();

COMMIT;
