// Package formats keeps the corpus of R3V's stored formats in
// testdata/formats: version records, trees and blobs (plain, zstd, stored
// behind the header, chunk lists and their pieces), each with what R3V
// reads from it, and bad ones it refuses. Readers outside R3V (R3V Cloud's
// web app) test against it; the test here checks that R3V reads the corpus
// as it says (go test -run Corpus -update writes it again). The formats are
// written up in docs/design/formats.md.
package formats
