// Package migrator provides migrator utilities.
//
// When loading from multiple directories or embedded filesystems, duplicate
// migration version+direction pairs are rejected instead of silently
// overriding one another. If commit acknowledgement is ambiguous after the SQL
// has executed, the runner records the migration as uncertain and blocks later
// runs until an operator reconciles the database state and migration record.
package migrator
