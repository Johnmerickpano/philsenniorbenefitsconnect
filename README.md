# PhilSenior Benefits Connect

Flutter + Go + ScyllaDB + Docker. Status: M0 (infrastructure only). Synthetic data only.

## Start
    copy .env.example .env
    docker compose up -d
    docker compose ps
    docker compose logs scylla-init

## Inspect
    docker exec -it psbc-scylla cqlsh

## Persistence test
    docker exec -it psbc-scylla cqlsh -e "CREATE TABLE IF NOT EXISTS psbc.persist_check (id int PRIMARY KEY, note text); INSERT INTO psbc.persist_check (id, note) VALUES (1, 'still here');"
    docker compose restart scylla
    docker exec -it psbc-scylla cqlsh -e "SELECT * FROM psbc.persist_check;"
    docker exec -it psbc-scylla cqlsh -e "DROP TABLE psbc.persist_check;"

## Stop / reset
    docker compose stop
    docker compose down          (keeps data)
    docker compose down -v       (DELETES local DB data)
