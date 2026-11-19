-- What a router does decides which jobs poll it and which tabs it shows.
-- Every router that exists today is a border router, so the default is the
-- backfill and no job rows change.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role varchar(8) NOT NULL DEFAULT 'border';
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role_set_by_admin boolean NOT NULL DEFAULT false;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role_guess varchar(8);
