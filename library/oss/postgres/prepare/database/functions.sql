-- Collection of optional helper functions
-- To provision these functions add
--   provision_helper_functions: true
-- to the database intent.

-- `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE … ADD COLUMN IF NOT EXISTS`
-- both require exclusive locks with Postgres, even if the table/column already exists.
-- The functions below provide ensure semantics while only acquiring exclusive locks on mutations.

-- fn_ensure_table is a lock-friendly replacement for `CREATE TABLE IF NOT EXISTS`.
--
-- Example usage:
--
-- SELECT fn_ensure_table('testtable', $$
--   UserID TEXT NOT NULL,
--   PRIMARY KEY(UserID)
-- $$);
CREATE OR REPLACE FUNCTION fn_ensure_table(tname TEXT, def TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_tables
    WHERE schemaname = 'public' AND tablename = LOWER(tname)
  ) THEN
    EXECUTE 'CREATE TABLE IF NOT EXISTS ' || tname || ' (' || def || ');';
  END IF;
END
$func$;

-- fn_ensure_column is a lock-friendly replacement for `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`.
--
-- Example usage:
--
-- SELECT fn_ensure_column('testtable', 'CreatedAt', 'TIMESTAMP DEFAULT CURRENT_TIMESTAMP');
CREATE OR REPLACE FUNCTION fn_ensure_column(tname TEXT, cname TEXT, def TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ADD COLUMN IF NOT EXISTS ' || cname || ' ' || def;
  END IF;
END
$func$;