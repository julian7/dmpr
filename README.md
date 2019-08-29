# DMPR: Database mapper from scratch

This database mapper is written for sqlx database library, mainly supporting postgres. It aims to be as light weight as possible.

[![Go Report Card](https://goreportcard.com/badge/github.com/julian7/dmpr)](https://goreportcard.com/report/github.com/julian7/dmpr)
[![GoDoc](https://godoc.org/github.com/julian7/dmpr?status.svg)](https://godoc.org/github.com/julian7/dmpr)
[![Releases](https://img.shields.io/github/release/julian7/dmpr/all.svg)](https://github.com/julian7/dmpr/releases)

## In Scope

* maintains a database connection
* provides logrus logging
* provides health report on the connection
* provides basic query functionality on top of sqlx for logging purposes
* provides basic model query functionality (Find, FindBy, All, Create, Update, Delete)
* provides basic "belongs to", "has one", and "has many" relationships (NewSelect)

## Out of Scope

* Transactions (for now)
* Cascading joins in select: all joins are referencing the original model only.

## Map models

Models are structs, and mapper reads their "db" tags for meta-information, just like sqlx. There are a couple of rule of thumbs, which might make your life easier:

* Database table names are generated by struct names by converting to snake_cased, pluralized form.
* Empty `db:"..."` tag names are not handled well. If there is a tag, it must be named.
* if the tag is "-" (just like in `db:"-"`), then that field will not be represented in the database.
* if the tag is missing, sqlx uses a standard mapping: field name converted to lower case, and never `snake_case` (wrt. table names).
* mapper accepts the following tag options (optional fields after a comma):
  * omitempty: if the field is empty in the model, it won't be added to Create / Update query
  * relation: it represents "has one" or "has many" relationships (depending on the field type)
  * belongs: represents "belongs_to" relationship. It assumes another field with the same name, but with `_id` suffix.
  * related maps can and should be added to structs. To avoid circular references, use pointers for related structs.
* References may accept both values or pointers. However, go doesn't accept circular value references. As a simple rule, I'd suggest you to use values at "belongs to", but use pointers at "has one" or "has many" relationships.

## Relations

### Belongs

When a struct "belongs to" another struct, it stores the other struct's ID like this:

```golang
type Message struct {
    ID    int
    Title string
    Body  string
}

type Comment struct {
    ID     int
    Title  string
    Body   string
    PostID int     `db:"post_id"`
    Post   Message `db:"post,belongs"`
}
```

In this case, Comment belongs to Message, and it's referenced internally as "post". It also requires a `post_id` field, as it will be stored in the table.

Selecting a Comment looks like this:

```golang
import "gitlab.com/julian7/dmpr"

comments := &[]Comment{}
query, err := dmpr.NewSelect(comments)
if err != nil {
    panic(err)
}
query.Where(dmpr.Eq("id", 1)).Join("post").All()
```

This query loads `comment` with an appropriate comment, with the data of `Post`, which is a `Message` object.

## Has one / has many

When a struct "has one" another struct, it stores the struct ID at the other struct:

```golang
type User struct {
    ID       int
    Name     string
    Password string
    Profile  *Profile `db:"profile,relation=user"`
}

type Profile struct {
    ID      int
    UserID  int  `db:"user_id"`
    Email   string
}
```

In this case, User "has a" Profile, but Profile doesn't "belong to" User. User requires a reference to a profile, and Profile requieres a `user_id` field. User's Profile field requires an option "relation" with a value how Profile is referencing it.

Selecting a User with profile looks like this:

```golang
import "gitlab.com/julian7/dmpr"

users := &[]User{}
query, err := dmpr.NewSelect(users)
if err != nil {
    panic(err)
}
query.Where(dmpr.Eq("id", 1)).Join("profile").All()
```

A "has_many" relationship is similar to "has_one", but the referencing struct is in a slice:

```golang
type ToDoList struct {
    ID         int
    Name       string
    ToDoItems []*ToDoItem `db:"to_do_items,relation=list"`
}

type UserGroup struct {
    ID      int
    ListID  int     `db:"list_id"`
    Name    string
}

toDoLists := &[]ToDoList{}
query, err := dmpr.NewSelect(users)
if err != nil {
    panic(err)
}
query.Join("to_do_items").All()
```

## Many to many

A "many to many" relation represents an _n:m_ relationship, with an anonymous linking table:

```sql
CREATE TABLE users (
    id SERIAL,
    name VARCHAR(32)
);

CREATE TABLE groups (
    id SERIAL,
    name VARCHAR(32)
);

CREATE TABLE user_groups (
    user_id INT,
    group_id INT
);

SELECT t1.id, t1.name, t2.id AS group_id, t2.name AS group_name
FROM users t1
LEFT JOIN user_groups tt2 ON (t1.id=tt2.user_id)
LEFT JOIN groups t2 ON (t2.id=tt2.group_id);
```

It is more compact in go:

```go
type User struct {
    ID   int
    Name string
    Groups []Group `db:groups,relation=user,reverse=group,through=user_groups"`
}

type Group struct {
    ID int
    Name string
    Users []*User `db:users,relation=group,reverse=user,through=user_groups"`
}

users := &[]User{}
query, err := dmpr.NewSelect(users)
if err != nil {
    panic(err)
}
query.Join("groups").All()

```

## Operators

There are just a couple of operators implemented, but it's very easy to add more. They work in a way

### Null operator

`dmpr.Null("column", true)` provides a "column IS NULL" operator. If the second parameter is `false`, then it will provide "column IS NOT NULL" instead.

### Eq operator

`dmpr.Eq(column, value)` provides an equivalence operator, in the form of `column = VALUE` or `column IN (value...)`.

### Not operator

`dmpr.Not(operator)` negates an operator. For example, `dmpr.Not(dmpr.Null("column", true))` returns `colum IS NOT NULL`.

### And operator

`dmpr.And(operator...)` groups other operators together, to provide a single operator with an AND relationship between them.

### Or operator

`dmpr.Or(operator...)` groups other operators together, to provide a single operator with an OR relationship between them.
