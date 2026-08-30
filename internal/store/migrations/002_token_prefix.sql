-- The leading characters of a token, stored so the interface can say which row
-- corresponds to the secret sitting in an agent's config file.
--
-- An agent that has had a token reissued has several rows with the same name,
-- and nothing else distinguishes them. A prefix of a 256-bit random secret
-- gives away nothing -- roughly 250 bits of entropy remain -- and it is the
-- only durable handle between a row here and a value in a file somewhere else.
ALTER TABLE token ADD COLUMN prefix TEXT NOT NULL DEFAULT '';
