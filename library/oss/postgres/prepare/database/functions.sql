-- Collection of optional helper functions
-- To provision these functions add
--   provision_helper_functions: true
-- to the database intent.

-- `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE … ADD COLUMN IF NOT EXISTS`
-- both require exclusive locks with Postgres, even if the table/column already exists.
-- The functions below provide ensure semantics while only acquiring exclusive locks on mutations.

-- fn_ensure_table is a lock-friendly replacement for `CREATE TABLE IF NOT EXISTS`.
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
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
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
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
    EXECUTE 'ALTER TABLE ' || tname || ' ADD COLUMN IF NOT EXISTS ' || cname || ' ' || def || ';';
  END IF;
END
$func$;

-- fn_ensure_column_not_exists is a lock-friendly replacement for `ALTER TABLE ... DROP COLUMN IF EXISTS`.
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
--
-- Example usage:
--
-- SELECT fn_ensure_column_not_exists('testtable', 'CreatedAt');
CREATE OR REPLACE FUNCTION fn_ensure_column_not_exists(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' DROP COLUMN IF EXISTS ' || cname || ';';
END IF;
END
$func$;

-- fn_ensure_column_not_null is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL`.
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
--
-- Example usage:
--
-- SELECT fn_ensure_column_not_null('testtable', 'Role');
CREATE OR REPLACE FUNCTION fn_ensure_column_not_null(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname) AND is_nullable = 'NO'
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ALTER COLUMN ' || cname || ' SET NOT NULL;';
  END IF;
END
$func$;

-- fn_ensure_column_nullable is a lock-friendly replacement for `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL`.
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
--
-- Example usage:
--
-- SELECT fn_ensure_column_nullable('testtable', 'Role');
CREATE OR REPLACE FUNCTION fn_ensure_column_nullable(tname TEXT, cname TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = LOWER(tname) AND column_name = LOWER(cname) AND is_nullable = 'NO'
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' ALTER COLUMN ' || cname || ' DROP NOT NULL;';
END IF;
END
$func$;

-- fn_ensure_replica_identity is a lock-friendly replacement for `ALTER TABLE ... REPLICA IDENTITY ...`.
-- WARNING: This function translates all names into lowercase (as plain postgres would).
-- If you want to use lowercase characters, (e.g. through quotation) do not use this function.
-- Does not support index identities.
--
-- Example usage:
--
-- SELECT fn_ensure_replica_identity('testtable', 'FULL');
CREATE OR REPLACE FUNCTION fn_ensure_replica_identity(tname TEXT, replident TEXT)
  RETURNS void
  LANGUAGE plpgsql AS
$func$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_class WHERE oid = tname::regclass AND CASE relreplident
          WHEN 'd' THEN 'default'
          WHEN 'n' THEN 'nothing'
          WHEN 'f' THEN 'full'
       END = LOWER(replident)
  ) THEN
    EXECUTE 'ALTER TABLE ' || tname || ' REPLICA IDENTITY ' || replident || ';';
  END IF;
END
$func$;