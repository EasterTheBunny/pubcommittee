// Package exports builds the M1 bulk exports and the change feed from stored
// data (design/10 § Bulk exports, § Change feed). Everything is rebuilt from
// source on each run and written through the Publisher-backed export and feed
// ports: per-version files are write-once (bill text and original) or replaced
// whole (cited law and explanations, which later runs may correct), the
// catalog export and the index are replaced each run, and the feed only ever
// gains entries. A run over unchanged data changes nothing.
package exports
