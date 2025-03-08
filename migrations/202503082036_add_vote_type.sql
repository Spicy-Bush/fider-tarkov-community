ALTER TABLE post_votes ADD COLUMN vote_type INTEGER NOT NULL DEFAULT 1;
CREATE INDEX post_votes_vote_type_idx ON post_votes(vote_type);
