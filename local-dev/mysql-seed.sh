#!/bin/sh
(
    set -eu
    for seed in /ctf-seed/*.sql; do
        [ -f "$seed" ] || continue
        MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --protocol=socket --user=root \
            --default-character-set=utf8mb4 --database="$MYSQL_DATABASE" < "$seed"
    done
)
