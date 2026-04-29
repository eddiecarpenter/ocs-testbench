// Package logging is the application's structured logging facade.
//
// It will wrap the project's chosen logger (slog / zap) so the rest of
// the codebase depends on a single shape once the surrounding Features
// land. At present the package exists so the module skeleton compiles;
// no public API has been defined yet.
package logging
