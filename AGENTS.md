# Rules

## language-go

### Anti-Patterns

#### 🎯 Directives
- NEVER use `init()` functions unless strictly necessary (e.g., registering drivers or database dialects). They obscure state, make tests harder, and execute unpredictably.
- NEVER use mutable global variables. Pass state explicitly through structs or functional options.
- NEVER fire-and-forget goroutines. ALWAYS wait for goroutines to exit using `sync.WaitGroup` or an errgroup to prevent goroutine leaks and ensure graceful shutdown.
- NEVER use naked parameters in function signatures (e.g., `printInfo("foo", true, true)`). Use boolean types with descriptive names, or use functional options / struct parameters for multiple booleans.
- NEVER spawn goroutines in `init()` functions.
- NEVER pass a mutex by value; ALWAYS pass by pointer `*sync.Mutex` or embed as a value in a struct passed by pointer.

#### 📝 Examples

##### ✅ DO
```go
// Avoid naked parameters
type PrintOptions struct {
	Verbose bool
	Debug   bool
}
func printInfo(name string, opts PrintOptions) { /* ... */ }

// Wait for goroutines
func processAll(items []int) {
	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			process(i)
		}(item)
	}
	wg.Wait()
}
```

##### ❌ DON'T
```go
// Don't use naked boolean parameters
func printInfo(name string, verbose bool, debug bool) { /* ... */ }
// Called as: printInfo("foo", true, true) - Unreadable!

// Don't fire and forget goroutines
func processAll(items []int) {
	for _, item := range items {
		go process(item) // Leak risk! No way to know when finished or if it errored.
	}
}

// Don't use init for logic
var db *sql.DB
func init() {
	db = connectDB() // Hard to test, executes immediately on import
}

// Don't use mutable globals
var ActiveConnections int // Mutable global, subject to race conditions!
```

### Architecture and Structure Standards

#### 🎯 Directives
- ALWAYS call `os.Exit` or `log.Fatal` ONLY in `main()`. Other packages should return errors.
- ALWAYS call `os.Exit` only once within `main()`. Use a separate `run()` function if complex setup is needed.
- ALWAYS use `defer` to clean up resources (files, locks) immediately after acquiring them.
- ALWAYS group similar declarations (constants, variables, types) together using block syntax `const (...)`.
- ALWAYS separate import groups: standard library, third-party packages, and local packages, separated by blank lines.
- NEVER put application logic inside `main()`. Delegate to a `run()` or `Start()` method.
- NEVER mix unrelated declarations in the same group block.
- NEVER set channel sizes arbitrarily. Channel size should be one or none (unbuffered).
- NEVER start enums at zero if zero is not a valid or intended default state. Start enums at one.

#### 📝 Examples

##### ✅ DO
```go
// Import grouping
import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"myproject/internal/service"
)

// Grouping declarations
const (
	defaultPort = 8080
	defaultTimeout = 5 * time.Second
)

type (
	Request struct{}
	Response struct{}
)

// Exit only in main
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// ... App logic
	return nil
}

// Enums starting at one
type Color int
const (
	ColorUnknown Color = iota
	ColorRed
	ColorBlue
)
```

##### ❌ DON'T
```go
// Don't mix import groups
import (
	"context"
	"myproject/internal/service"
	"fmt"
	"github.com/pkg/errors"
)

// Don't scatter declarations
const defaultPort = 8080
const defaultTimeout = 5 * time.Second

// Don't exit in packages
func doWork() {
	if err := something(); err != nil {
		log.Fatal(err) // NEVER do this outside main
	}
}

// Don't use arbitrary channel sizes
c := make(chan int, 64) // Why 64? Size 1 or 0 is preferred.
```

### Code Style and Formatting Standards

#### 🎯 Directives
- ALWAYS reduce nesting by returning early from functions. Handle errors first.
- ALWAYS avoid unnecessary `else` blocks if the `if` block returns or breaks.
- ALWAYS use short variable declarations `:=` if a variable is being set to some value explicitly.
- ALWAYS keep the scope of variables as small as possible. Use `if err := ...; err != nil {` if the variable is only needed in the `if` block.
- ALWAYS declare slice and map variables as `nil` slices rather than initialized empty slices if they might remain empty (e.g., `var s []string` instead of `s := []string{}`).
- ALWAYS use raw string literals (backticks) to avoid manual escaping of quotes and regexes.
- NEVER create unformatted code; ALWAYS format with `gofmt` or `goimports`.
- NEVER declare format strings inside the print function if they are static constants. Format strings outside of `Printf` should be `const`.
- NEVER use overly long lines; break them into multiple lines for readability.

#### 📝 Examples

##### ✅ DO
```go
// Reduce nesting with early returns
func calculate(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, ErrEmpty
	}
	
	// Main logic here, no else
	return len(b), nil
}

// Reduce scope of variables
func write() error {
	if err := os.WriteFile("file.txt", []byte("hello"), 0644); err != nil {
		return err
	}
	return nil
}

// Nil slice is a valid slice
var items []string
if condition {
	items = append(items, "test")
}

// Raw strings
regex := regexp.MustCompile(`\.txt$`)
```

##### ❌ DON'T
```go
// Don't use unnecessary else
func calculate(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, ErrEmpty
	} else {
		return len(b), nil
	}
}

// Don't expand scope unnecessarily
func write() error {
	err := os.WriteFile("file.txt", []byte("hello"), 0644)
	if err != nil {
		return err
	}
	return nil
}

// Don't initialize empty slices unless necessary
items := []string{}
if condition {
	items = append(items, "test")
}

// Don't manually escape strings if raw literals are cleaner
regex := regexp.MustCompile("\\.txt$")
```

### Configuration and Environment Standards

#### 🎯 Directives
- ALWAYS use field tags (e.g., `json:"field_name"`, `yaml:"field_name"`) in structs that are marshaled or unmarshaled to explicit format. 
- ALWAYS handle time using the `"time"` package (`time.Time` for instants, `time.Duration` for spans).
- ALWAYS specify units in field names when dealing with configurations that do not support `time.Duration` natively (e.g., `TimeoutMillis int` instead of `Timeout int`).
- NEVER use global variables to store configuration. Pass configuration into the components that need it, often via a Config struct.
- NEVER ignore errors when parsing configuration from the environment or files.

#### 📝 Examples

##### ✅ DO
```go
// Specifying units in JSON configs where Duration isn't supported
type Config struct {
	Port          int `json:"port"`
	TimeoutMillis int `json:"timeout_millis"` // Explicit unit
}

// Using time package for typed configs
type Server struct {
	Timeout time.Duration
}

// Passing config explicitly
func NewServer(cfg Config) *Server {
	return &Server{
		Timeout: time.Duration(cfg.TimeoutMillis) * time.Millisecond,
	}
}
```

##### ❌ DON'T
```go
// Don't omit struct tags on exported marshaled structs
type Config struct {
	Port    int
	Timeout int // Ambiguous unit! Is it seconds? ms?
}

// Don't use globals for config
var GlobalConfig Config // Hard to mock, mutable!

func doWork() {
	timeout := GlobalConfig.Timeout
}
```

### Dependency Management Standards

#### 🎯 Directives
- ALWAYS use Go modules (`go.mod` and `go.sum`) for dependency management.
- ALWAYS run `go mod tidy` before committing to ensure `go.mod` and `go.sum` accurately reflect the actual dependencies used in the code.
- ALWAYS vendor dependencies (`go mod vendor`) only if required by project policy or CI/CD constraints; otherwise, rely on the module cache.
- NEVER modify `go.mod` manually to add dependencies. Use `go get` or `go mod tidy`.
- NEVER commit a `go.mod` with `replace` directives pointing to local absolute paths (e.g., `replace github.com/user/pkg => /Users/name/code/pkg`) to the main branch, as it breaks builds for other developers.

#### 📝 Examples

##### ✅ DO
```bash
### Add a dependency
go get github.com/stretchr/testify

### Tidy before commit
go mod tidy
```

##### ❌ DON'T
```go
// Don't commit local replace directives in go.mod
module myapp

go 1.21

require github.com/user/pkg v1.0.0

// NEVER commit this to shared branches!
replace github.com/user/pkg => /home/dev/local/pkg
```

### Documentation and Comments Standards

#### 🎯 Directives
- ALWAYS write a package comment for all packages. For multi-file packages, place the package comment in a `doc.go` file or the primary file.
- ALWAYS document all exported (public) identifiers (variables, constants, functions, methods, types) with a comment that begins with the identifier's name.
- ALWAYS format comments as complete sentences, ending with a period.
- NEVER use comments to explain "how" code works if the code itself can be refactored to be clear. Comments should explain "why".
- NEVER leave exported functions undocumented, even if their purpose seems obvious.

#### 📝 Examples

##### ✅ DO
```go
// Package math provides basic constants and mathematical functions.
//
// This package does not guarantee bit-for-bit identical results across architectures.
package math

// Parser reads and parses a configuration file.
type Parser struct {
	// ...
}

// Parse loads the configuration from the given file path.
// It returns an error if the file cannot be read or contains invalid syntax.
func (p *Parser) Parse(path string) error {
	// ...
	return nil
}
```

##### ❌ DON'T
```go
package math // Missing package comment

type Parser struct { // Exported type missing comment
}

// parses the file
func (p *Parser) Parse(path string) error { // Comment must start with the function name "Parse" and be a full sentence.
	return nil
}
```

### Error Handling Standards

#### 🎯 Directives
- ALWAYS return errors as the last argument of a function.
- ALWAYS use `fmt.Errorf` with the `%w` verb to wrap errors if adding context, or use custom error types for structured errors.
- ALWAYS handle errors exactly once. Logging an error and then returning it is handling it twice.
- ALWAYS use the "comma ok" idiom for type assertions to avoid panics.
- NEVER use `panic` in normal application code. Errors should be returned to the caller.
- NEVER use a bare `except` or generic panic recovery unless at the absolute top-level of a goroutine to prevent the process from crashing.
- NEVER swallow errors implicitly using `_ = err`. If an error is explicitly ignored, ALWAYS document why with a comment.

#### 📝 Examples

##### ✅ DO
```go
// Wrapping errors with context
func readFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	return b, nil
}

// Handling type assertion safely
t, ok := i.(string)
if !ok {
	return fmt.Errorf("expected string, got %T", i)
}

// Handling errors once
func doSomething() error {
	if err := something(); err != nil {
		return err // Return without logging
	}
	return nil
}
```

##### ❌ DON'T
```go
// Don't panic
func readFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err) // NEVER panic in normal code
	}
	return b
}

// Don't handle type assertions unsafely
t := i.(string) // Will panic if i is not a string

// Don't handle errors twice
func doSomething() error {
	if err := something(); err != nil {
		log.Printf("error doing something: %v", err)
		return err // Handled twice!
	}
	return nil
}
```

### Logging and Observability Standards

#### 🎯 Directives
- ALWAYS handle an error exactly once: either log it, or return it, but never both.
- ALWAYS use structured logging (e.g., `slog`, `zap`, `logrus`) rather than standard `log` or `fmt` for production applications.
- ALWAYS use thread-safe mechanisms (like `go.uber.org/atomic`) for telemetry counters and gauges to prevent race conditions.
- NEVER use `log.Fatal` or `log.Panic` outside of the `main()` function. Libraries should return errors.
- NEVER log sensitive information (PII, credentials, tokens) in plain text.

#### 📝 Examples

##### ✅ DO
```go
// Return error to be handled by caller
func connect() error {
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

// Log and handle at the top boundary
func main() {
	if err := connect(); err != nil {
		logger.Error("startup failed", zap.Error(err))
		os.Exit(1)
	}
}

// Use atomic for thread-safe counters
type Metrics struct {
	Requests atomic.Int64
}
```

##### ❌ DON'T
```go
// Don't log and return
func connect() error {
	if err := db.Ping(); err != nil {
		log.Printf("error: database ping failed: %v", err) // Handled here...
		return err // ...and handled here! (Double logging)
	}
	return nil
}

// Don't use Fatal in libraries
func parseConfig() {
	if err := read(); err != nil {
		log.Fatal(err) // Kills the entire application unexpectedly
	}
}
```

### Naming Conventions Standards

#### 🎯 Directives
- ALWAYS name packages using lowercase, single-word names (e.g., `time`, `list`, `http`).
- ALWAYS name interfaces with 'er' suffixes when they contain a single method (e.g., `Reader`, `Writer`).
- ALWAYS name error variables starting with `Err` for exported variables (e.g., `ErrNotFound`) and `err` for unexported (e.g., `errInternal`).
- ALWAYS name error types with an `Error` suffix (e.g., `NotFoundError`).
- ALWAYS prefix unexported package-level global variables with an underscore `_` to make it clear they are global.
- ALWAYS use camelCase for local variables and functions, and PascalCase for exported identifiers.
- NEVER use built-in names (e.g., `error`, `string`, `int`) as variable or struct field names to avoid shadowing.
- NEVER use `Get` as a prefix for getter methods (use `User()` instead of `GetUser()`).

#### 📝 Examples

##### ✅ DO
```go
// Error naming
var ErrCouldNotOpen = errors.New("could not open")

type NotFoundError struct {
	err error
}

func (e *NotFoundError) Error() string { return e.err.Error() }

// Unexported globals prefix
var _defaultPort = 8080

// Avoiding built-in shadowing
func handleErrorMessage(msg string) error {
	var err error // builtin 'error' is intact
	return err
}

// Getters without "Get"
func (c *Client) Timeout() time.Duration {
	return c.timeout
}
```

##### ❌ DON'T
```go
// Don't use non-standard error names
var ErrorCouldNotOpen = errors.New("could not open")

type NotFound struct { // Missing Error suffix
	err error
}

// Don't prefix unexported globals without underscore
var defaultPort = 8080 // Ambiguous if local or global

// Don't shadow built-ins
func handleErrorMessage(error string) {
	// 'error' shadows the builtin type
}

// Don't use "Get" prefix for getters
func (c *Client) GetTimeout() time.Duration {
	return c.timeout
}
```

### Performance and Optimization Standards

#### 🎯 Directives
- ALWAYS specify container capacity (slices, maps) during initialization using `make(type, length, capacity)` or `make(map[T]V, capacity)` when the size is known or can be estimated.
- ALWAYS prefer `strconv` over `fmt.Sprint` or `fmt.Sprintf` for primitive type conversions (e.g., `strconv.Itoa`, `strconv.FormatBool`), as it is significantly faster.
- ALWAYS use `atomic` operations (prefer `go.uber.org/atomic`) instead of mutexes for simple primitive counters or flags.
- NEVER perform repeated string-to-byte or byte-to-string conversions in a loop or hot path.
- NEVER use `fmt.Sprintf` for simple string concatenations; use `+` or `strings.Builder`.
- NEVER unnecessarily allocate memory for `time.Timer` or `time.Ticker` in a loop; reuse them by resetting.

#### 📝 Examples

##### ✅ DO
```go
// Pre-allocate slice capacity
func getIds(users []User) []int {
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

// Use strconv for primitives
func buildKey(id int) string {
	return "user:" + strconv.Itoa(id)
}

// Avoiding repeated string/byte conversions
func isValid(b []byte) bool {
	// Use bytes package directly
	return bytes.HasPrefix(b, []byte("http://"))
}
```

##### ❌ DON'T
```go
// Don't leave capacity unspecified when known
func getIds(users []User) []int {
	var ids []int // Will reallocate multiple times
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

// Don't use fmt for primitive conversions
func buildKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

// Don't convert in hot paths unnecessarily
func isValid(s string) bool {
	// Converts string to byte slice on every call
	return bytes.HasPrefix([]byte(s), []byte("http://"))
}
```

### Security and Validation Standards

#### 🎯 Directives
- ALWAYS validate inputs at the public API boundary of your package or service.
- ALWAYS use the "comma ok" idiom for map lookups and type assertions to prevent nil dereferences and panics on untrusted input.
- ALWAYS use thread-safe data structures (`sync.Map`, `sync.RWMutex`, `atomic` types) when accessing data concurrently. Maps in Go are not thread-safe.
- NEVER construct SQL queries using `fmt.Sprintf` or string concatenation. ALWAYS use parameterized queries or prepared statements to prevent SQL injection.
- NEVER use insecure random number generators (`math/rand`) for security-sensitive tokens, passwords, or keys. ALWAYS use `crypto/rand`.

#### 📝 Examples

##### ✅ DO
```go
// Safe map lookups
if val, ok := myMap["key"]; ok {
	process(val)
}

// Parameterized SQL query
func getUser(db *sql.DB, id string) (*User, error) {
	row := db.QueryRow("SELECT name FROM users WHERE id = ?", id)
	// ...
}

// Secure random generation
func generateToken() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
```

##### ❌ DON'T
```go
// Don't do unsafe map lookups that might crash or return invalid zero values
val := myMap["key"] // Unsafe if you expect it to exist
process(val)

// Don't construct SQL queries with strings
func getUser(db *sql.DB, id string) (*User, error) {
	// SQL Injection vulnerability!
	query := fmt.Sprintf("SELECT name FROM users WHERE id = '%s'", id)
	row := db.QueryRow(query)
	// ...
}

// Don't use math/rand for security
func generateToken() string {
	return fmt.Sprintf("%d", mathRand.Int63()) // Insecure!
}
```

### Testing Standards

#### 🎯 Directives
- ALWAYS use table-driven tests for testing multiple scenarios of the same logic. It reduces duplication and makes adding new test cases trivial.
- ALWAYS name your test table variable `tests` or `tt` and use a slice of anonymous structs for the test cases.
- ALWAYS use `t.Run(tt.name, func(t *testing.T) { ... })` for each test case in a table-driven test to ensure proper sub-test isolation and reporting.
- NEVER use assertions that panic the test runner (like custom panics) on failure. Use `t.Fatal`, `t.Fatalf`, `t.Error`, or `t.Errorf`.
- NEVER share state across sub-tests. If using shared state, ensure `t.Parallel()` is either not used or handled safely.

#### 📝 Examples

##### ✅ DO
```go
// Table-driven tests
func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		host, port string
		wantErr    bool
	}{
		{
			name:    "valid",
			in:      "localhost:8080",
			host:    "localhost",
			port:    "8080",
			wantErr: false,
		},
		{
			name:    "missing port",
			in:      "localhost",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := net.SplitHostPort(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SplitHostPort() error = %v, wantErr %v", err, tt.wantErr)
			}
			if host != tt.host || port != tt.port {
				t.Errorf("got %q:%q, want %q:%q", host, port, tt.host, tt.port)
			}
		})
	}
}
```

##### ❌ DON'T
```go
// Don't duplicate test logic for each case
func TestSplitHostPort_Valid(t *testing.T) {
	host, port, err := net.SplitHostPort("localhost:8080")
	// assertions...
}

func TestSplitHostPort_MissingPort(t *testing.T) {
	_, _, err := net.SplitHostPort("localhost")
	// assertions...
}
```

### Type Safety Standards

#### 🎯 Directives
- ALWAYS use pointers to structs `*MyStruct` to represent optionality, mutation, or large struct passing. 
- ALWAYS use values for slices and maps, as they are already reference types.
- ALWAYS verify interface compliance at compile time using the `var _ MyInterface = (*MyStruct)(nil)` idiom.
- ALWAYS use `var` for zero-value structs (e.g., `var buf bytes.Buffer`), but use `:=` with `make` or literals when initializing with non-zero values. (Zero-value mutexes `sync.Mutex` are valid and should not be initialized via pointer unless embedded/passed).
- ALWAYS use field names to initialize structs, and omit zero-value fields.
- ALWAYS copy slices and maps at API boundaries if they are mutated.
- NEVER use pointers to interfaces (e.g., `*error` or `*io.Reader`). An interface is already a pointer under the hood.
- NEVER embed types in public structs unless you intend to expose all the methods of the embedded type to the public API.
- NEVER return unexported types from an exported function or method.

#### 📝 Examples

##### ✅ DO
```go
// Verifying interface compliance
type Handler struct{}
var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

// Zero-value struct initialization
var buf bytes.Buffer
buf.Write([]byte("hello"))

// Struct Initialization
type User struct {
	Name    string
	Age     int
	IsAdmin bool
}
u := User{
	Name: "Alice",
	Age:  30, // IsAdmin omitted because zero-value is false
}

// Pass interfaces as values (not pointers)
func process(r io.Reader) error {
	// ...
}
```

##### ❌ DON'T
```go
// Don't forget compile-time interface compliance checks
type Handler struct{}
// Missing: var _ http.Handler = (*Handler)(nil)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

// Don't use new() or struct{} for zero values unnecessarily
buf := new(bytes.Buffer)
// or
buf := bytes.Buffer{}

// Don't init structs without field names
u := User{"Alice", 30, false} // Fragile to schema changes!

// Don't use pointers to interfaces
func process(r *io.Reader) error { // interface is already a reference type
	// ...
}

// Don't embed in public structs without intention
type AbstractList struct {}
func (l *AbstractList) InternalAdd() {} // Will be exposed!

type ConcreteList struct {
	AbstractList // Exposes InternalAdd
}
```


# Code Context Engine (Probe)

Probe is configured for this workspace. Use Probe MCP tools to inspect and search code dynamically across target folder paths instead of raw static AST dumps:
- `probe search "<query>" [path]` - Search code semantically with Elasticsearch-style syntax.
- `probe extract <file>:<line>` - Extract complete AST semantic blocks.
- `probe query "<pattern>"` - Perform AST structural pattern matching.
- `probe symbols <file>` - List code symbols (functions, classes, constants) in target file.
