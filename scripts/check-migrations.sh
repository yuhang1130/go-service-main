#!/bin/sh
set -eu

if [ ! -d migrations ]; then
  exit 0
fi

duplicates=$(find migrations -type f -name '*.up.sql' -exec basename {} \; | cut -d_ -f1 | sort | uniq -d)
if [ -n "$duplicates" ]; then
  echo "duplicate migration versions:"
  echo "$duplicates"
	exit 1
fi

for migration in migrations/*.up.sql; do
	[ -e "$migration" ] || continue
	down=${migration%.up.sql}.down.sql
	if [ ! -f "$down" ]; then
		echo "missing down migration for: $migration"
		exit 1
	fi
done

for migration in migrations/*.down.sql; do
	[ -e "$migration" ] || continue
	up=${migration%.down.sql}.up.sql
	if [ ! -f "$up" ]; then
		echo "missing up migration for: $migration"
		exit 1
	fi
done
