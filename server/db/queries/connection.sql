
-- name: InsertConnection :exec
INSERT INTO connections (
    remote_addr
) VALUES (
    $1
);

-- name: DeleteConnection :exec
DELETE FROM connections WHERE remote_addr = $1;