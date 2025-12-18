# injector

**injector** is a command-line tool that generates type-safe dependency injection (DI) code for Go projects.

Unlike traditional DI frameworks that rely on provider sets or complex wiring DSLs, injector uses **Go’s type system** and a minimal marker tag to describe dependency injection in a clear and explicit way.

The core idea is simple:

> - The Container declares what is injected. 
> - Providers declare how values are constructed.

---

## Install

Install `injector` using `go install`:

```bash
go install github.com/mickamy/injector@latest
```

---

## Core Concepts

### 1. Container

A **Container** is a struct that declares the components your application exposes. Fields marked with the `inject` tag are managed by injector.

```go
type Container struct {
	Service service.User `inject:""`
}

```

- The `inject` tag is **purely a marker**.
- It carries no value or configuration by default.
- It simply signals: *"this field is managed by injector."*

### 2. Providers

A **provider** is any top-level function that constructs a value.

Go

```go
func NewUser(database *infra.Database) User {
	return &user{database: database}
}
```

- Providers are **automatically discovered** during static analysis.
- Dependencies are **inferred from function parameters**.
- No manual registration or wiring code is required.

---

## Why a Marker Tag?

The marker-only `inject` tag serves several important purposes:

- Makes injected fields **explicit and auditable**.
- Prevents accidental injection of unrelated struct fields.
- Clearly identifies the struct acting as the Container.
- Provides a foundation for future extensions without breaking compatibility.

The tag is intentionally minimal:

```go
Service service.User `inject:""`
```

No values. No DSL. No complex configuration.

---

## Interface-First Design

injector is designed to work naturally with interfaces, allowing you to expose **only interfaces** in your Container while keeping implementations private.

```go
type User interface {
	Register(name, password string) error
}

type user struct {
	DB infra.Database
}

func (u *user) Register(name string, password string) error {
	log.Printf("insert user %s with password %s\n", name, password)
	return nil
}

func NewUser(db infra.Database) User {
	return &user{DB: db}
}
```

- The concrete type remains **unexported**.
- The provider returns the interface.
- The Container depends only on the interface, promoting decoupling.

---

## Full Example

See the full example in the `example` directory.

---

## Code Generation

Run the generator:


```bash
injector generate ./...
```

This produces code similar to the following:

```go
func NewContainer() *Container {
	cfg := config.NewDatabase()
	db := infra.NewDatabase(cfg)
	srv := service.NewUser(db)

	return &Container{
		Service: srv,
	}
}
```

---

## Provider Selection

By default, injector selects a provider **by its return type**. If exactly one provider returns the required type, it is used automatically.

When multiple providers exist for the same type, you can explicitly select one using the `provider` directive.

```go
type Container struct {
	Service service.UserService `inject:"provider:service.NewUser"`
}
```

- `provider:<FuncName>` specifies the exact constructor function to use.
- The provider’s return type must match the field type.
- Dependencies are still automatically resolved from the provider’s parameters.

---

## Provider Overrides for Internal Components

Sometimes a dependency required by another component has multiple providers, even if that dependency itself is not exposed by the Container. In such cases, you can declare a **provider override** in the Container using a blank (`_`) field.

```go
type Container struct {
    _ infra.Database `inject:"provider:infra.NewReaderDatabase"`
    User service.UserService `inject:""`
}
```

- The blank field **does not expose** the component publicly.
- It defines a global provider selection rule for that specific type within the container.
- Any resolution requiring `infra.Database` will now use `NewReaderDatabase`.

This keeps provider selection centralized while preserving a clean public API.

---

## Dependency Resolution Rules

- **A valid provider** is a function that:
  - Has no receiver (top-level function).
  - Returns exactly one value.
* **Dependencies are resolved from:**
  - The provider function specified by `inject:"provider:<FuncName>"`.
  - The unique provider function that matches the required type.
- **Selection Logic:**
  - Automatic if a single provider matches the type.
  - Explicit `inject:"provider:<FuncName>"` required if multiple providers match.
- **Generation Fails if:**
  - A dependency has no provider.
  - Ambiguous providers exist without an explicit directive.
  - Cyclic dependencies are detected.

---

## Design Philosophy

- Prefer **Go’s type system** over external configuration.
- Make dependency injection **explicit but minimal**.
- Keep application logic **free from DI-specific code**.
- **Centralize wiring** in a single, readable location.
- **Fail fast** at generation time.

---

## License

[MIT](https://www.google.com/search?q=./LICENSE)