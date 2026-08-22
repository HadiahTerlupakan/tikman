#!/bin/sh
# Installs the repo's git hooks into .git/hooks/ (not tracked by git, so each
# clone needs this run once).
#
# Appends rather than overwrites: graphify installs its own hooks in the same
# directory, and core.hooksPath is deliberately left unset so both survive.
set -e

hooks_dir=$(git rev-parse --git-path hooks)
mkdir -p "$hooks_dir"

for hook in pre-commit; do
    src="scripts/$hook"
    dest="$hooks_dir/$hook"

    if [ -f "$dest" ] && ! grep -q "tikman-hook" "$dest" 2>/dev/null; then
        echo "$hook already exists and is not ours; not overwriting it." >&2
        echo "Merge scripts/$hook into $dest by hand." >&2
        continue
    fi

    {
        echo "#!/bin/sh"
        echo "# tikman-hook"
        tail -n +2 "$src"
    } > "$dest"
    chmod +x "$dest"
    echo "installed $dest"
done
