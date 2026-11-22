# HW 6

## Запуск

Нужны Go 1.26+, PostgreSQL и Redis. Из каталога Go-модуля:

```sh
docker run -d --name go-hw-postgres -e POSTGRES_USER=movies -e POSTGRES_PASSWORD=movies -e POSTGRES_DB=movies -p 5432:5432 postgres:17
docker run -d --name go-hw-redis -p 6379:6379 redis:7
export DATABASE_URL='postgres://movies:movies@localhost:5432/movies?sslmode=disable'
export REDIS_ADDR='localhost:6379'
go run ./cmd/movie-service
```

При старте приложение автоматически применяет SQL-миграции из `internal/adapter/postgres/migrations`, синхронизирует отсутствующие фильмы со стендом и строит индекс по всем фильмам PostgreSQL. HTTP и gRPC запускаются после завершения этих действий. Повторная загрузка не создаёт дубликаты.

## Переменные окружения

| Переменная | Назначение | Значение по умолчанию |
| --- | --- | --- |
| `MOVIE_BASE_URL` | URL фильм-стенда | `https://homeworksite.site` |
| `HTTP_ADDR` | Адрес HTTP | `:8080` |
| `GRPC_ADDR` | Адрес gRPC | `:9090` |
| `DATABASE_URL` | DSN PostgreSQL | Обязательная |
| `REDIS_ADDR` | Адрес Redis без пароля | `localhost:6379` |

## Поиск и кэш

Индекс Redis: `go_hw:index:<слово>` — Set идентификаторов фильмов без TTL. Слова выделяются как последовательности Unicode-букв и цифр из `title` и `director` и приводятся к нижнему регистру.

Многословный поиск использует AND: Redis `SINTER` находит фильмы, содержащие все слова запроса в названии или имени режиссёра. Поиск идёт по целым словам. Фильмы по найденным идентификаторам читаются из PostgreSQL одним запросом.

Ключ ответа: `go_hw:cache:<SHA-256 запроса после удаления крайних пробелов>`. Значение — полный JSON-массив фильмов, TTL — 3600 секунд с момента записи. Чтение не продлевает TTL. Повторный запрос возвращает ответ из Redis без обращения к PostgreSQL. Пустой результат тоже кэшируется; ошибки не кэшируются.

При каждом старте индекс заменяется и старый кэш ответов удаляется. После очистки Redis следующий запуск восстановит индекс из PostgreSQL, включая ранее загруженные фильмы. Приложению нужен отдельный Redis либо выделенные префиксы `go_hw:index:` и `go_hw:cache:`.

## Запросы

```sh
curl --get --data-urlencode 'q=Solaris' http://localhost:8080/search
curl --get --data-urlencode 'q=Andrei Tarkovsky' http://localhost:8080/search
curl --get --data-urlencode 'q=SOLARIS' http://localhost:8080/search
curl 'http://localhost:8080/load-movies?from=1&to=3'
curl http://localhost:8080/last-movie
```

Пустой запрос — HTTP 400. Успешный поиск — HTTP 200 и массив `id`, `title`, `year`, `director`; без совпадений — `[]`. Ошибка хранилища — HTTP 500.

Swagger: `http://localhost:8080/swagger/index.html`. gRPC сохраняет методы `LoadMovies` и `GetLastMovie` из `api/movies/v1/movies.proto`.

## Тесты

```sh
go test -race ./...
TEST_DATABASE_URL="$DATABASE_URL" TEST_REDIS_ADDR="$REDIS_ADDR" go test -race -p 1 ./...
```

Для интеграционных тестов используйте отдельный тестовый PostgreSQL и Redis: тесты создают временные схемы и перестраивают поисковый индекс.
