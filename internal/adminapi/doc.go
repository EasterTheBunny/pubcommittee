// Package adminapi is the handler of admin-api, the privileged service
// operators reach through pubctl ([architecture § Administration]). It is not
// part of the public contract and sends no CORS headers.
//
// Every mutating call needs a non-empty reason and writes a public log entry
// that names the operator (id and public name, never a Google principal). A
// change that is one document is written with PlatformDocs.ApplyLogged, so an
// action whose log write fails is not applied; a change that spans documents
// (re-ingest) writes its log entry first and runs only if that succeeded.
//
// The package never reads individual signals or request reasons: no such
// port exists in M1, and test/ci/deps.sh keeps them out of admin-api's graph.
package adminapi
