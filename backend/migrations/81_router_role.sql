-- What a router does decides which jobs poll it and which tabs it shows.
-- Every router that exists today is a border router, so the default is the
-- backfill and no job rows change.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role varchar(8) NOT NULL DEFAULT 'border';
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role_set_by_admin boolean NOT NULL DEFAULT false;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS role_guess varchar(8);
-- Routers that exist today stay on the role they have until an admin picks
-- Otomatis: discovery would otherwise move a live border router to its guess
-- the first time it runs after the deploy. Only routers added later start
-- with the guess in charge.
UPDATE mikrotik_routers SET role_set_by_admin = true;
