CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE conversation_states(
    chat_id VARCHAR(255) UNIQUE NOT NULL,
    last_question_asked VARCHAR(255) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(chat_id)
);

CREATE TRIGGER conversation_states
BEFORE UPDATE ON conversation_states
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();