# gt

`gt` is a command line tool that performs convenient actions on a
gnucash sqlite3 database.

**warning: gt can destroy your gnucash sqlite3 database**

`gt` is not associated with the gnucash project in anyway. There is no
guarantee that any action performed by `gt` will not corrupt, destroy or
modify in an unexpected way a gnucash sqlite3 database. If you choose to
use `gt`, please backup your gnucash sqlite3 database before doing so.

## Why?

I had completed a migration of transactions into gnucash and needed a
way to bulk edit many transactions. At the time, there was no support
for bulk editing transactions within gnucash. There are libraries that
provide access to the gnucash sqlite3 database though I had written
enough code that worked on the gnucash sqlite3 database to be
comfortable in writing my own tool.

## How?

Build:
```shell
$ go build -o ./gt ./cmd/gt/main.go
```

The `gt` config file defaults to `~/.gt.json` which supports these
options:
```json
{
    "gnucash_db_file": "/home/user/.gnucash.sql.gnucash"
}
```

List accounts:
```shell
$ gt account list
```

List accounts that have _Groceries_ in their name:
```shell
$ gt account list --name-like "Groceries"
```

Get an account by `guid`:
```shell
$ gt account get 9b1d2bc513da4076b236aee6114b21a7
```

Get an account by account tree:
```shell
$ gt account get expenses:groceries
```

List transactions:
```shell
$ gt transaction list
```

List transactions for account `guid`:
```shell
$ gt transaction list --account 9b1d2bc513da4076b236aee6114b21a7
```

List transactions for account tree:
```shell
$ gt transaction list --account expenses:groceries
```

List transactions starting from a date with a description that contains
_%Pizza_:
```shell
$ gt transaction list --start-post-date 2024-01-01 \
    --description-like "%Pizza"
```

Update a transaction account:
```shell
$ gt transaction update 0000000000000000fa1ce5381fec0d51 \
    --source-account expenses:pizza
    --destination-account expenses:dining
```

Update many transactions account based on their description:
```shell
$ gt transaction bulk-update \
    --description-like "%Pizza" \
    --source-account expenses:pizza \
    --destination-account expenses:dining
```
