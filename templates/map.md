<!-- vistan: map -->

# Project map

What this project is, in one line.

## Entry points

- `src/main.ts`: where the process starts.

## Modules

- `src/api/`: HTTP handlers.
  - `client.ts`: talks to the upstream API.
- `src/db/`: queries and migrations.

## Data flow

- request -> handler -> db -> response

## Gotchas

- The thing that will bite whoever touches this next.
